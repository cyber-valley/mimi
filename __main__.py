import argparse
import functools
import logging
from concurrent.futures import ThreadPoolExecutor
from datetime import timedelta
from enum import StrEnum, auto
from pathlib import Path
from queue import Queue
from typing import assert_never

from mimi import DataOrigin, data_scraper
from mimi.data_scraper import DataScraperMessage
from mimi.data_scraper.github import GithubScraperContext
from mimi.data_scraper.telegram import TelegramScraperContext
from mimi.data_scraper.x import XScraperContext

logging.basicConfig(level=logging.DEBUG)
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
    should_raise = True
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
            poll_interval = (
                timedelta(seconds=args.poll_interval) if args.poll_interval else None
            )
            scraper = functools.partial(
                data_scraper.x.scrape,
                XScraperContext(
                    user_tweets_json_directory=args.user_tweets_json_directory,
                    accounts_to_follow=args.accounts_to_follow,
                    poll_interval=poll_interval,
                ),
            )
            should_raise = poll_interval is not None
        case DataOrigin.TELEGRAM:
            parser.add_argument(
                "--group-names",
                "-n",
                nargs="+",
                help="List of telegram groups to search",
            )
            parser.add_argument(
                "--history-depth",
                "-h",
                required=True,
                help="Amount of existing messages to process on the start",
            )
            parser.add_argument(
                "--process-new", "-p", action=argparse.BooleanOptionalAction
            )
            args = parser.parse_args()
            scraper = functools.partial(
                data_scraper.telegram.scrape,
                TelegramScraperContext(
                    group_names=args.group_names,
                    history_depth=args.history_depth,
                    process_new=args.process_new,
                ),
            )
            should_raise = args.process_new is not None
        case DataOrigin.GITHUB:
            parser.add_argument(
                "--port",
                "-p",
                type=int,
                default=8000,
                help="Port for the webhook server to listen on.",
            )
            parser.add_argument(
                "--repository-base-path",
                "-b",
                type=Path,
                required=True,
                help="Base path for storing the repositories.",
            )
            parser.add_argument(
                "--repositories-to-follow",
                "-r",
                nargs="+",
                required=True,
                help="List of GitHub repositories to follow (owner/repo).",
            )
            args = parser.parse_args()
            scraper = functools.partial(
                data_scraper.github.scrape,
                GithubScraperContext(
                    port=args.port,
                    repository_base_path=args.repository_base_path,
                    repositories_to_follow={
                        data_scraper.github.GitRepository(*repo.split("/"))
                        for repo in args.repositories_to_follow
                    },
                ),
            )
        case _:
            assert_never(args.data_origin)

    sink: Queue[DataScraperMessage] = Queue()

    with ThreadPoolExecutor() as pool:
        futures = data_scraper.run_scrapers(pool, sink, [scraper])
        for future in futures:
            try:
                future.result()
            except (
                data_scraper.x.XScraperStopped,
                data_scraper.telegram.TelegramScraperStopped,
            ):
                if should_raise:
                    raise

    sink.shutdown()
    _consume_queue(sink)


def _consume_queue(queue: Queue[DataScraperMessage]) -> None:
    while not queue.empty():
        log.info(queue.get())


if __name__ == "__main__":
    main()
