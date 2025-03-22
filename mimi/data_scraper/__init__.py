from collections.abc import Awaitable, Callable, Iterable
from dataclasses import dataclass
from datetime import datetime
from multiprocessing.pool import ThreadPool
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


def run_scrapers(
    pool: ThreadPool,
    sink: DataSink[DataScraperMessage],
    scrapers: Iterable[DataScraper],
) -> NoReturn:
    for scraper in scrapers:
        pool.apply_async(scraper, (sink,))
    raise ScraperStoppedError
