import logging
from collections.abc import Callable, Iterable
from concurrent.futures import Future, ThreadPoolExecutor
from dataclasses import dataclass
from datetime import datetime
from typing import NoReturn

import tenacity
from tenacity import retry

from mimi import DataOrigin, DataSink

log = logging.getLogger(__name__)


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
    wrap_retry = retry(
        before_sleep=tenacity.before_sleep_log(log, logging.ERROR, exc_info=True),
        after=tenacity.after_log(log, logging.INFO),
        wait=tenacity.wait_exponential(multiplier=1, max=10),
    )
    return [executor.submit(wrap_retry(scraper), sink) for scraper in scrapers]
