import asyncio
import logging
import os
from collections.abc import AsyncIterator, Collection
from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import NoReturn

from result import Err, Ok, Result
from telethon import Client, types
from telethon._impl import tl
from telethon._impl.session import ChannelRef
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
    async for dialog in client.get_dialogs():
        if dialog.chat.id not in group_ids:
            log.debug(
                "Got unconfigured chat %s with id %s", dialog.chat.name, dialog.chat.id
            )
            continue
        log.info(
            "Processing %s chat with type %s, id %s and depth %s",
            dialog.chat.name,
            type(dialog.chat),
            dialog.chat.id,
            depth,
        )
        if isinstance(dialog.chat, types.Group) and dialog.chat.is_megagroup:
            # XXX: There is no limit for the forum topics amount
            # so I assume that 100 will be enough
            messages = await client.get_messages(dialog.chat, limit=100)
            channel_ref = dialog.chat.ref
            assert isinstance(channel_ref, ChannelRef)
            forum_topics = await client(
                tl.functions.channels.get_forum_topics_by_id(
                    channel=channel_ref._to_input_channel(),  # noqa: SLF001
                    topics=[
                        message.replied_message_id
                        for message in messages
                        if message.replied_message_id is not None
                    ],
                )
            )
            log.info("Got response %s", forum_topics)
        else:
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
