from asyncio import gather
from collections.abc import Awaitable, Callable, Iterable
from dataclasses import dataclass
from datetime import datetime
from typing import NoReturn

from mimi import DataOrigin, DataSink


class ScraperStoppedError(Exception):
    pass


@dataclass
class DataScraperMessage:
    data: str
    origin: DataOrigin
    scraped_at: datetime


DataScraper = Callable[[DataSink[DataScraperMessage]], Awaitable[NoReturn]]


async def run_scrapers(
    sink: DataSink[DataScraperMessage], scrapers: Iterable[DataScraper]
) -> NoReturn:
    await gather(*[scraper(sink) for scraper in scrapers])
    raise ScraperStoppedError
