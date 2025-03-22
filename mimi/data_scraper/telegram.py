import asyncio
import functools
import logging
from dataclasses import dataclass
from typing import NoReturn

from telethon import Client, events

from mimi import DataOrigin, DataSink

from . import DataScraperMessage

log = logging.getLogger(__name__)
TelegramMessage = functools.partial(DataScraperMessage, origin=DataOrigin.TELEGRAM)


class TelegramScraperStopped(Exception):  # noqa: N818
    pass


@dataclass
class TelegramScraperContext:
    client_name: str
    api_id: int
    api_hash: str
    bot_api_token: str
    group_names: set[str]


def scrape(
    context: TelegramScraperContext, sink: DataSink[DataScraperMessage]
) -> NoReturn:
    asyncio.run(_start_client(context, sink))


async def _start_client(
    context: TelegramScraperContext, sink: DataSink[DataScraperMessage]
) -> NoReturn:
    client = Client(context.client_name, context.api_id, context.api_hash)
    await client.interactive_login(context.bot_api_token)

    @client.on(events.NewMessage)
    async def handle_new_message(event: events.NewMessage) -> None:
        chat = await event.get_chat()
        if chat.name not in context.group_names:
            return
        log.info(event)

    client.start()
    client.run_until_disconnected()

    raise TelegramScraperStopped
