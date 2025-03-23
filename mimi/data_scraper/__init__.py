from collections.abc import Callable, Iterable
from concurrent.futures import Future, ThreadPoolExecutor
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
    pub_date: datetime
    identifier: str


DataScraper = Callable[[DataSink[DataScraperMessage]], NoReturn]


def run_scrapers(
    executor: ThreadPoolExecutor,
    sink: DataSink[DataScraperMessage],
    scrapers: Iterable[DataScraper],
) -> list[Future[NoReturn]]:
    return [executor.submit(scraper, sink) for scraper in scrapers]
