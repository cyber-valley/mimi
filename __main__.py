import argparse
import atexit
import functools
import json
import logging
import logging.config
import logging.handlers
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import timedelta
from enum import StrEnum, auto
from pathlib import Path
from queue import Queue
from typing import Final, NoReturn, assert_never

from mimi import (
    DataOrigin,
    data_scraper,
    embedding_pipeline,
    factory,
    rag_chat_prompt,
    telegram_bot,
)
from mimi.config import EmbeddingPipelineConfig, TelegramConfig
from mimi.data_scraper import DataScraperMessage
from mimi.data_scraper.github import GithubScraperContext
from mimi.data_scraper.telegram import PeersConfig, TelegramScraperContext
from mimi.data_scraper.x import XScraperContext

log = logging.getLogger(__name__)

DEFAULT_CONFIG_FILE_NAME: Final = "mimi_config.json"


class Command(StrEnum):
    SCRAPE = auto()
    EMBEDDING_PIPELINE = auto()
    TELEGRAM_BOT = auto()
    COMPLETE = auto()


def main() -> None:
    setup_logging()
    parser = argparse.ArgumentParser(
        description="Mimi President which scrapes data and provides RAG powered chat."
    )
    parser.add_argument("command", choices=list(Command), type=Command)

    args, _ = parser.parse_known_args()
    match args.command:
        case Command.SCRAPE:
            execute_scraper(parser)
        case Command.EMBEDDING_PIPELINE:
            execute_embedding_pipeline(parser)
        case Command.TELEGRAM_BOT:
            execute_telegram_bot(parser)
        case Command.COMPLETE:
            execute_complete(parser)
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
                "--groups-ids",
                "-g",
                nargs="+",
                help="List of telegram groups to search.",
            )
            parser.add_argument(
                "--forums-ids",
                "-f",
                nargs="+",
                help="List of telegram forums to search.",
            )
            parser.add_argument(
                "--history-depth",
                default=5,
                help="Amount of existing messages to process on the start.",
            )
            parser.add_argument(
                "--process-new",
                "-p",
                action=argparse.BooleanOptionalAction,
                help="Listen to the new messages.",
            )
            args = parser.parse_args()
            scraper = functools.partial(
                data_scraper.telegram.scrape,
                TelegramScraperContext(
                    peers_config=PeersConfig(
                        groups_ids=set(map(int, args.groups_ids or [])),
                        forums_ids=set(map(int, args.forums_ids or [])),
                    ),
                    history_depth=args.history_depth,
                    process_new=args.process_new,
                    last_sync_date=None,
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
            parser.add_argument(
                "--run-server",
                action=argparse.BooleanOptionalAction,
                help="Listen to GitHub webhooks.",
            )
            args = parser.parse_args()
            scraper = functools.partial(
                data_scraper.github.scrape,
                GithubScraperContext(
                    port=args.port,
                    run_server=args.run_server,
                    repository_base_path=args.repository_base_path,
                    repositories_to_follow={
                        data_scraper.github.GitRepository(*repo.split("/"))
                        for repo in args.repositories_to_follow
                    },
                ),
            )
            should_raise = args.run_server
        case _:
            assert_never(args.data_origin)

    sink: Queue[DataScraperMessage] = Queue()

    with ThreadPoolExecutor() as pool:
        futures = data_scraper.run_scrapers(pool, sink, [scraper], enable_retry=False)
        for future in futures:
            try:
                future.result()
            except (
                data_scraper.x.XScraperStopped,
                data_scraper.telegram.TelegramScraperStopped,
                data_scraper.github.GithubScraperStopped,
            ):
                if should_raise:
                    raise

    sink.shutdown()
    while not sink.empty():
        log.info(sink.get())


class EmbeddingStoppedError(Exception):
    pass


def execute_embedding_pipeline(parser: argparse.ArgumentParser) -> NoReturn:
    parser.add_argument(
        "--config",
        "-c",
        type=Path,
        default=DEFAULT_CONFIG_FILE_NAME,
        help="Config file to run pipeline.",
    )
    args = parser.parse_args()
    config = EmbeddingPipelineConfig.from_dict(json.loads(args.config.read_text()))
    scrapers = []
    if config.scrapers.github:
        scrapers.append(
            functools.partial(data_scraper.github.scrape, config.scrapers.github)
        )
    if config.scrapers.x:
        scrapers.append(functools.partial(data_scraper.x.scrape, config.scrapers.x))
    if config.scrapers.telegram:
        scrapers.append(
            functools.partial(data_scraper.telegram.scrape, config.scrapers.telegram)
        )

    sink: Queue[DataScraperMessage] = Queue()
    connection = factory.get_connection(config.embedding.db_file)
    vector_store = factory.get_vector_store(
        connection,
        config.embedding.embedding_provider,
        config.embedding.embedding_model_name,
        config.embedding.embedding_table_name,
    )

    with ThreadPoolExecutor(max_workers=len(scrapers) + 2) as pool:
        futures = (
            pool.submit(
                embedding_pipeline.run,
                sink,
                connection,
                vector_store,
                config.embedding.embedding_table_name,
            ),
            pool.submit(_queue_size_listener, sink),
            *data_scraper.run_scrapers(pool, sink, scrapers),
        )
        for future in as_completed(futures):
            print(future.result())
            raise EmbeddingStoppedError

    raise EmbeddingStoppedError


def _queue_size_listener[T](sink: Queue[T]) -> NoReturn:
    while True:
        log.info("Queue size %s", sink.qsize())
        time.sleep(60)


def execute_telegram_bot(parser: argparse.ArgumentParser) -> NoReturn:
    parser.add_argument(
        "--config",
        "-c",
        type=Path,
        default=DEFAULT_CONFIG_FILE_NAME,
        help="Config file to run pipeline.",
    )
    args = parser.parse_args()
    config = TelegramConfig.from_dict(json.loads(args.config.read_text()))
    connection = factory.get_connection(config.embedding.db_file)
    vector_store = factory.get_vector_store(
        connection,
        config.embedding.embedding_provider,
        config.embedding.embedding_model_name,
        config.embedding.embedding_table_name,
    )
    llm = factory.get_llm(config.llm.provider, config.llm.model)
    graph = rag_chat_prompt.init(vector_store, llm, config.llm.max_documents_to_find)
    telegram_bot.run(graph)


def execute_complete(parser: argparse.ArgumentParser) -> None:
    parser.add_argument(
        "--config",
        "-c",
        type=Path,
        default=DEFAULT_CONFIG_FILE_NAME,
        help="Config file to run pipeline.",
    )
    parser.add_argument(
        "query",
        help="Query to complete with LLM.",
    )
    args = parser.parse_args()
    config = TelegramConfig.from_dict(json.loads(args.config.read_text()))
    connection = factory.get_connection(config.embedding.db_file)
    vector_store = factory.get_vector_store(
        connection,
        config.embedding.embedding_provider,
        config.embedding.embedding_model_name,
        config.embedding.embedding_table_name,
    )
    llm = factory.get_llm(config.llm.provider, config.llm.model)
    graph = rag_chat_prompt.init(vector_store, llm, config.llm.max_documents_to_find)
    print(rag_chat_prompt.complete(graph, args.query))


def setup_logging() -> None:
    config = json.loads(Path("logging.json").read_text())
    logging.config.dictConfig(config)
    queue_handler = logging.getHandlerByName("queue_handler")
    if queue_handler is None:
        return
    assert isinstance(queue_handler, logging.handlers.QueueHandler)
    assert queue_handler.listener is not None
    queue_handler.listener.start()
    atexit.register(queue_handler.listener.stop)


if __name__ == "__main__":
    main()
