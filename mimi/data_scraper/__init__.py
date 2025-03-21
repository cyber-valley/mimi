from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from datetime import datetime
from typing import NoReturn

from mimi import DataOrigin, DataSink


@dataclass
class DataScraperMessage:
    data: str
    origin: DataOrigin
    scraped_at: datetime


DataScraper = Callable[[DataSink[DataScraperMessage]], Awaitable[NoReturn]]
