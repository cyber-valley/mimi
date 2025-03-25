import asyncio
import logging
import os
from collections.abc import AsyncIterator, Collection
from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import NoReturn

from result import Err, Ok, Result
from telethon import Client, types
from telethon._impl import tl, client
from telethon._impl.session import ChannelRef, GroupRef
from telethon.events import Event, NewMessage
from telethon.types import Message

from mimi import DataOrigin, DataSink

from . import DataScraperMessage

log = logging.getLogger(__name__)


class TelegramScraperStopped(Exception):  # noqa: N818
    pass


@dataclass
class TelegramScraperContext:
    group_ids: set[int]
    history_depth: int
    process_new: bool
    client_name: str = field(default_factory=lambda: os.environ["TELEGRAM_CLIENT_NAME"])
    api_id: int = field(default_factory=lambda: int(os.environ["TELEGRAM_API_ID"]))
    api_hash: str = field(default_factory=lambda: os.environ["TELEGRAM_API_HASH"])

    # We don't want to expose private data
    # or mess with special logging formatter
    def __repr__(self) -> str:
        return object.__repr__(self)

    def __post_init__(self) -> None:
        assert self.group_ids, "Group ids shouldn't be empty."


def scrape(
    context: TelegramScraperContext, sink: DataSink[DataScraperMessage]
) -> NoReturn:
    asyncio.run(_scrape(context, sink))


async def _scrape(
    context: TelegramScraperContext, sink: DataSink[DataScraperMessage]
) -> NoReturn:
    log.info("Starting Telegram scraper")
    client = Client(context.client_name, context.api_id, context.api_hash)
    await client.connect()
    await client.interactive_login()

    log.info("Downloading history")
    update_counter = 0
    async for message in _download_updates(
        client, context.group_ids, context.history_depth
    ):
        sink.put(message)
        update_counter += 1

    log.info("Downloaded %s updates", update_counter)

    if context.process_new:
        log.info("Starting Telegram new messages listening")
        await _process_updates(client, sink, context.group_ids)

    raise TelegramScraperStopped


async def _download_updates(
    client: Client, group_ids: Collection[int], depth: int
) -> AsyncIterator[DataScraperMessage]:
    for id in group_ids:
        log.info("Starting to scrape group %s", id)
        peer, *_ = await client.resolve_peers([ChannelRef(id)])
        log.info("Resolved peer %s", peer)
        forum_topics = await client(
                tl.functions.channels.get_forum_topics(
                    channel=peer.ref._to_input_channel(),  # noqa: SLF001
                    q="",
                    offset_date=0,
                    offset_id=0,
                    offset_topic=0,
                    limit=100
                )
            )
        log.info("Got response %s", forum_topics)

        async for message in client.get_messages(dialog.chat, limit=depth):
            match _convert_to_internal_message(message):
                case Ok(msg):
                    yield msg
                case Err(text):
                    log.warning(
                        "Failed to parse message %s with text %s",
                        message.id,
                        text,
                    )


async def _process_updates(
    client: Client, sink: DataSink[DataScraperMessage], group_ids: Collection[int]
) -> NoReturn:
    @client.on(NewMessage)
    async def _(event: Event) -> None:
        assert isinstance(event, NewMessage)
        if event.chat.id not in group_ids:
            return

        log.info("Got new message from %s: %s", event.chat.name, event)

        match _convert_to_internal_message(event):
            case Ok(msg):
                sink.put(msg)
            case Err(text):
                log.warning(text)

    await client.run_until_disconnected()

    raise TelegramScraperStopped


def _convert_to_internal_message(message: Message) -> Result[DataScraperMessage, str]:
    if not message.text:
        return Err("Empty message text")

    if not message.date:
        return Err("Empty message date")

    return Ok(
        DataScraperMessage(
            data=message.text,
            identifier=f"{message.chat.id}:{message.id}",
            origin=DataOrigin.TELEGRAM,
            pub_date=message.date,
            scraped_at=datetime.now(UTC),
        )
    )
