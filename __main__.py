import argparse
import functools
import logging
from datetime import timedelta
from enum import StrEnum, auto
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from queue import Queue
from typing import assert_never

from mimi import DataOrigin, data_scraper
from mimi.data_scraper import DataScraperMessage
from mimi.data_scraper.x import XScraperContext

logging.basicConfig(level=logging.INFO)
log = logging.getLogger(__name__)


class Command(StrEnum):
    SCRAPE = auto()


def main() -> None:
    parser = argparse.ArgumentParser(description="Scrape data")
    parser.add_argument("command", choices=list(Command), type=Command)

    args, _ = parser.parse_known_args()
    match args.command:
        case Command.SCRAPE:
            execute_scraper(parser)
        case _:
            assert_never(args.command)


def execute_scraper(parser: argparse.ArgumentParser) -> None:
    parser.add_argument(
        "data_origin",
        type=DataOrigin,
        choices=list(DataOrigin),
        help="Source of the data being scraped",
    )
    args, _ = parser.parse_known_args()
    match args.data_origin:
        case DataOrigin.X:
            parser.add_argument(
                "--user-tweets-json-directory",
                "-d",
                required=True,
                type=Path,
                help="Path to the directory containing user tweets JSON files.",
            )
            parser.add_argument(
                "--accounts-to-follow",
                "-a",
                nargs="+",
                required=True,
                help="List of X accounts to follow.",
            )
            parser.add_argument(
                "--poll-interval",
                "-s",
                type=int,
                default=None,
                help=(
                    "Polling interval in seconds. If not provided, the scraper"
                    "will run once and exit."
                ),
            )
            args = parser.parse_args()
            poll_interval = timedelta(seconds=args.poll_interval) if args.poll_interval else None
            context = XScraperContext(
                user_tweets_json_directory=args.user_tweets_json_directory,
                accounts_to_follow=args.accounts_to_follow,
                poll_interval=poll_interval,
            )
            scraper = functools.partial(data_scraper.x.scrape, context)
        case DataOrigin.TELEGRAM:
            raise NotImplementedError
        case DataOrigin.GITHUB:
            raise NotImplementedError
        case _:
            assert_never(args.data_origin)

    sink: Queue[DataScraperMessage] = Queue()

    with ThreadPoolExecutor() as pool:
        futures = data_scraper.run_scrapers(pool, sink, [scraper])
        for future in futures:
            try:
                future.result()
            except data_scraper.x.XScraperStopped:
                if poll_interval is not None:
                    raise

    sink.shutdown()
    _consume_queue(sink)


def _consume_queue(queue: Queue[DataScraperMessage]) -> None:
    while True:
        log.info(queue.get())


if __name__ == "__main__":
    main()
