import asyncio
import logging
import os
from collections.abc import AsyncIterator, Collection
from dataclasses import dataclass, field
from datetime import UTC, datetime, timedelta
from typing import Any, NoReturn, assert_never, cast

from result import Err, Ok, Result
from telethon import TelegramClient
from telethon.events.newmessage import NewMessage
from telethon.tl.functions.channels import GetForumTopicsRequest
from telethon.tl.types import PeerChannel, PeerChat, PeerUser
from telethon.types import InputChannel, InputPeerChannel, Message

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
    client: TelegramClient, config: PeersConfig, depth: int
) -> AsyncIterator[DataScraperMessage]:
    delay = timedelta(milliseconds=100)
    for stream in (
        _scrape_groups(client, config.groups_ids, depth, delay),
        _scrape_forums(client, config.forums_ids, depth, delay),
    ):
        async for message in stream:
            match _convert_to_internal_message(message):
                case Ok(msg):
                    log.debug("Successfully parsed message with %s symbols", len(msg.data))
                    yield msg
                case Err(text):
                    log.warning(
                        "Failed to parse message %s with text %s",
                        message.id,
                        text,
                    )


async def _scrape_groups(
    client: TelegramClient, ids: Collection[int], depth: int, delay: timedelta
) -> AsyncIterator[Message]:
    for idx, group_id in enumerate(ids):
        log.info("[%s/%s]: Starting to scrape group %s", idx + 1, len(ids), group_id)
        input_peer_group = await client.get_input_entity(PeerChat(group_id))
        async for message in client.iter_messages(input_peer_group, limit=depth):
            yield message
        await asyncio.sleep(delay.total_seconds())


async def _scrape_forums(
    client: TelegramClient, ids: Collection[int], depth: int, delay: timedelta
) -> AsyncIterator[Message]:
    for idx, forum_id in enumerate(ids):
        log.info("[%s/%s]: Starting to scrape forum %s", idx + 1, len(ids), forum_id)

        input_peer_channel = await client.get_input_entity(PeerChannel(forum_id))
        assert isinstance(input_peer_channel, InputPeerChannel), (
            f"Got unexpected peer type f{input_peer_channel}"
        )

        topics_result = await client(
            GetForumTopicsRequest(
                channel=cast(InputChannel, input_peer_channel),
                offset_date=None,
                offset_id=0,
                offset_topic=0,
                limit=100,
                q=None,
            )
        )

        for topic in topics_result.topics:
            log.info("Starting scraping topic %s", topic.title)
            async for message in client.iter_messages(
                input_peer_channel,
                limit=depth,
                reply_to=topic.id,
            ):
                yield message

        await asyncio.sleep(delay.total_seconds())


async def _process_updates(
    client: TelegramClient,
    sink: DataSink[DataScraperMessage],
    peers_config: PeersConfig,
) -> NoReturn:
    @client.on(NewMessage)  # type: ignore
    async def _(event: Any) -> None:
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


def _convert_to_internal_message(message: Message) -> Result[DataScraperMessage, str]:
    if not message.message:
        return Err("Empty message text")

    if not message.date:
        return Err("Empty message date")

    peer_id: int
    if isinstance(message.peer_id, PeerUser):
        peer_id = message.peer_id.user_id
    elif isinstance(message.peer_id, PeerChat):
        peer_id = message.peer_id.chat_id
    elif isinstance(message.peer_id, PeerChannel):
        peer_id = message.peer_id.channel_id
    else:
        assert_never(message.peer_id)

    return Ok(
        DataScraperMessage(
            data=message.message,
            identifier=f"{peer_id}:{message.id}",
            origin=DataOrigin.TELEGRAM,
            pub_date=message.date,
            scraped_at=datetime.now(UTC),
        )
    )
