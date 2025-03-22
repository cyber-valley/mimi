from typing import NoReturn

from .data_scraper import DataScraperMessage
from .data_sink import DataSink


def run_embedding_pipeline(sink: DataSink[DataScraperMessage]) -> NoReturn:
    """
    Listens to updates from the `sink` and for each:

    - Saves raw data into database
    - Calculates and saves embedding
    - Checks delta between now and `scraped_at` to track late delivery
    """
    raise NotImplementedError
