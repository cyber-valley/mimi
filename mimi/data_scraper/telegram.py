import pprint
import inspect
import asyncio
import logging
import os
from collections.abc import AsyncIterator, Collection
from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import NoReturn, assert_never

from result import Err, Ok, Result
from telethon import Client
from telethon._impl import tl
from telethon._impl.session import ChannelRef, GroupRef
from telethon.events import Event, NewMessage
from telethon.types import Message

from mimi import DataOrigin, DataSink

from . import DataScraperMessage

log = logging.getLogger(__name__)


class TelegramScraperStopped(Exception):  # noqa: N818
    pass


@dataclass
class PeersConfig:
    groups_ids: set[int]
    forums_ids: set[int]
    all_ids: list[int] = field(init=False)

    def __post_init__(self) -> None:
        self.all_ids = [*self.groups_ids, *self.forums_ids]
        assert self.all_ids, "Got empty peers config."


@dataclass
class TelegramScraperContext:
    peers_config: PeersConfig
    history_depth: int
    process_new: bool
    client_name: str = field(default_factory=lambda: os.environ["TELEGRAM_CLIENT_NAME"])
    api_id: int = field(default_factory=lambda: int(os.environ["TELEGRAM_API_ID"]))
    api_hash: str = field(default_factory=lambda: os.environ["TELEGRAM_API_HASH"])

    # We don't want to expose private data
    # or mess with special logging formatter
    def __repr__(self) -> str:
        return object.__repr__(self)


def scrape(
    context: TelegramScraperContext, sink: DataSink[DataScraperMessage]
) -> NoReturn:
    asyncio.run(_scrape(context, sink))


async def _scrape(
    context: TelegramScraperContext, sink: DataSink[DataScraperMessage]
) -> NoReturn:
    log.info("Starting Telegram scraper")

    client = TelegramClient(context.client_name, context.api_id, context.api_hash)
    await client.start()  # type: ignore
    await client.connect()

    log.info("Downloading history")
    update_counter = 0
    async for message in _scrape_updates(
        client, context.peers_config, context.history_depth
    ):
        sink.put(message)
        update_counter += 1

    log.info("Downloaded %s updates", update_counter)

    if context.process_new:
        log.info("Starting Telegram new messages listening")
        await _process_updates(client, sink, context.peers_config)

    raise TelegramScraperStopped


async def _scrape_updates(
    client: Client, config: PeersConfig, depth: int
) -> AsyncIterator[DataScraperMessage]:
    for stream in (
        _scrape_groups(client, config.groups_ids, depth),
        _scrape_forums(client, config.forums_ids, depth),
    ):
        async for message in stream:
            match _convert_to_internal_message(message):
                case Ok(msg):
                    yield msg
                case Err(text):
                    log.warning(
                        "Failed to parse message %s with text %s",
                        message.id,
                        text,
                    )


async def _scrape_groups(
    client: Client, ids: Collection[int], depth: int
) -> AsyncIterator[Message]:
    for idx, group_id in enumerate(ids):
        log.info("[%s/%s]: Starting to scrape group %s", idx + 1, len(ids), group_id)
        # Because they have own abscrations it's tricky to handle error on each message
        try:
            async for message in client.get_messages(GroupRef(group_id), limit=depth):
                yield message
        # RPC Errors generated in runtime, so any exception should be
        # catched and examined
        except Exception as e:
            if getattr(e, "_code", -1) == 400:
                log.exception("Failed to get group's messages")
                continue
            raise


async def _scrape_forums(
    client: Client, ids: Collection[int], depth: int
) -> AsyncIterator[Message]:
    for idx, forum_id in enumerate(ids):
        log.info("[%s/%s]: Starting to scrape forum %s", idx + 1, len(ids), forum_id)

        match await client.resolve_peers([ChannelRef(forum_id)]):
            case [peer] if isinstance(peer.ref, ChannelRef):
                try:
                    forum_topics = await client(
                        tl.functions.channels.get_forum_topics(
                            channel=peer.ref._to_input_channel(),  # noqa: SLF001
                            q="",
                            offset_date=0,
                            offset_id=0,
                            offset_topic=0,
                            limit=100,
                        )
                    )
                # RPC Errors generated in runtime, so any exception should be
                # catched and examined
                except Exception as e:
                    if getattr(e, "_code", -1) == 400:
                        log.exception("Failed to get forum topics with")
                        continue
                    raise
            case other:
                log.error("Got unexcpected peers %s", other)
                continue

        assert hasattr(forum_topics, "count")
        assert hasattr(forum_topics, "topics")
        if forum_topics.count > len(forum_topics.topics):
            log.warning(
                "Not all topics were loaded (%s/%s).",
                len(forum_topics.topics),
                forum_topics.count,
            )

        for chat in forum_topics.chats:
            log.info("Got chat %s from topics", chat.id)

        for message, topic in zip(forum_topics.messages, forum_topics.topics):
            log.info("Got topic %s and peer_id %s / %s", topic.title, message.peer_id, peer.ref)
            try:
                messages_response = await client(
                    tl.functions.messages.get_replies(
                        peer=peer.ref._to_input_channel(),
                        msg_id=topic.id,
                        offset_id=0,
                        offset_date=0,
                        add_offset=0,
                        limit=depth,
                        max_id=0,
                        min_id=0,
                        hash=0
                    )
                )
            # RPC Errors generated in runtime, so any exception should be
            # catched and examined
            except Exception as e:
                if getattr(e, "_code", -1) == 400:
                    log.exception("Failed to get topic messages with")
                    continue
                raise

            assert hasattr(messages_response, "messages")
            log.info("Got %s message from topic", len(messages_response.messages))
            for message in messages_response.messages:
                yield message


async def _process_updates(
    client: Client, sink: DataSink[DataScraperMessage], peers_config: PeersConfig
) -> NoReturn:
    @client.on(NewMessage)
    async def _(event: Event) -> None:
        assert isinstance(event, NewMessage)
        if event.chat.id not in peers_config.all_ids:
            return

        log.info("Got new message from %s: %s", event.chat.name, event)

        match _convert_to_internal_message(event):
            case Ok(msg):
                log.debug("Message sended to the sink")
                sink.put(msg)
            case Err(text):
                log.warning(text)

    await client.run_until_disconnected()

    raise TelegramScraperStopped


def _convert_to_internal_message(message: Message | tl.types.MessageEmpty) -> Result[DataScraperMessage, str]:
    match message:
        case tl.types.MessageEmpty():
            return Err("Empty message")
        case Message(text) if not text:
            return Err("Empty message text")
        case Message(date) if not date:
            return Err("Empty message date")
        case Message(text):
          return Ok(
              DataScraperMessage(
                  data=message.text,
                  identifier=f"{message.chat.id}:{message.id}",
                  origin=DataOrigin.TELEGRAM,
                  pub_date=message.date,
                  scraped_at=datetime.now(UTC),
              )
          )
        case _:
            assert_nerver(message)
