import hashlib
import inspect
import logging
import sqlite3
from collections.abc import Iterable, Iterator
from contextlib import contextmanager
from dataclasses import dataclass, field
from datetime import UTC
from enum import StrEnum, auto
from os import PathLike
from pathlib import Path
from typing import NoReturn, assert_never, cast, override

import sqlite_vec
import tenacity
from langchain_community.vectorstores import VectorStore
from langchain_community.vectorstores.sqlitevec import SQLiteVec
from langchain_openai import OpenAIEmbeddings
from langchain_text_splitters import RecursiveCharacterTextSplitter, TextSplitter

from .data_scraper import DataScraperMessage
from .data_sink import DataSink

log = logging.getLogger(__name__)


class EmbeddingType(StrEnum):
    OPENAI = auto()


@dataclass
class EmbeddingPipelineContext:
    db_file: Path
    embedding_type: EmbeddingType
    embedding_model_name: str


@tenacity.retry(
    retry=tenacity.retry_if_not_exception_type(sqlite3.OperationalError),
    wait=tenacity.wait_exponential(multiplier=1, max=10),
    before_sleep=tenacity.before_sleep_log(log, logging.ERROR, exc_info=True),
    after=tenacity.after_log(log, logging.INFO),
)
def run(
    context: EmbeddingPipelineContext, sink: DataSink[DataScraperMessage]
) -> NoReturn:
    """
    Listens to updates from the `sink` and for each:
    - Saves raw data into the database
    - Calculates and saves embedding
    """
    log.info("Starting embedding pipeline")
    embedding_table_name: str = "embedding"

    match context.embedding_type:
        case EmbeddingType.OPENAI:
            embeddings = OpenAIEmbeddings(model=context.embedding_model_name)
            log.info("Using OpenAI embeddings %s", context.embedding_model_name)
        case _:
            assert_never(context.embedding_type)

    text_splitter = RecursiveCharacterTextSplitter(chunk_size=1000, chunk_overlap=200)
    connection = _create_connection(db_file=context.db_file)
    vectorstore = SQLiteVec(
        embedding_table_name,
        _create_connection_proxy(connection, _TranscationlessConnectionProxy),
        embeddings,
    )
    log.info("Starting transaction")
    with _transaction(connection):
        _create_tables_if_not_exists(connection)
        vectorstore.create_table_if_not_exists()
    log.info("Connected to database")

    while True:
        log.debug("Waiting for the new message")
        message = sink.get()
        _process_message(
            text_splitter, connection, vectorstore, embedding_table_name, message
        )


def _create_connection(db_file: PathLike[str]) -> sqlite3.Connection:
    connection = sqlite3.connect(db_file, autocommit=False)
    connection.row_factory = sqlite3.Row
    connection.enable_load_extension(True)  # noqa: FBT003
    sqlite_vec.load(connection)
    connection.enable_load_extension(False)  # noqa: FBT003
    return connection


def _create_connection_proxy[T: sqlite3.Connection](
    connection: sqlite3.Connection, proxy_cls: type[T]
) -> T:
    return cast(T, _ConnectionProxy(connection, proxy_cls))


@dataclass
class _ConnectionProxy[T: sqlite3.Connection]:
    obj: sqlite3.Connection
    proxy_cls: type[T]
    overridden_methods: dict[str, object] = field(init=False)

    def __post_init__(self) -> None:
        self.overridden_methods = {
            name: member
            for name, member in inspect.getmembers(self.proxy_cls)
            if hasattr(member, "__override__")
        }

    def __getattr__(self, name: str) -> object:
        return self.overridden_methods.get(name, getattr(self.obj, name))


class _TranscationlessConnectionProxy(sqlite3.Connection):
    @override
    def commit() -> None:  # type: ignore[misc]
        pass


@tenacity.retry(
    wait=tenacity.wait_exponential(multiplier=1, max=10),
    stop=tenacity.stop_after_attempt(3),
    before_sleep=tenacity.before_sleep_log(log, logging.ERROR, exc_info=True),
    after=tenacity.after_log(log, logging.INFO),
    retry_error_callback=lambda _: None,
)
def _process_message(
    text_splitter: TextSplitter,
    connection: sqlite3.Connection,
    vectorstore: VectorStore,
    embedding_table_name: str,
    message: DataScraperMessage,
) -> None:
    metadata = {
        "origin": message.origin,
        "scraped_at": int(message.scraped_at.replace(tzinfo=UTC).timestamp()),
        "pub_date": int(message.pub_date.replace(tzinfo=UTC).timestamp()),
    }
    splits = text_splitter.split_text(message.data)
    identifier_hash = hashlib.sha256(message.identifier.encode()).hexdigest()
    log.debug("Processing message from %s", message.origin)

    rowids = _find_rowids(identifier_hash, connection)
    with _transaction(connection):
        if rowids:
            _delete_embeddings(
                embedding_table_name, rowids, identifier_hash, connection
            )
            log.info("Deleted %s embeddings", len(rowids))

        rowids = vectorstore.add_texts(
            texts=splits,
            metadatas=[metadata for _ in splits],
        )
        _save_identifier_to_rowid(rowids, identifier_hash, connection)
    log.debug("Saved %s embeddings", len(rowids))


def _find_rowids(identifier_hash: str, connection: sqlite3.Connection) -> list[str]:
    return [
        row["rowid"]
        for row in connection.execute(
            """
            SELECT rowid FROM identifier_to_rowid
            WHERE hash = ?
            """,
            (identifier_hash,),
        )
    ]


def _delete_embeddings(
    table_name: str,
    rowids: Iterable[str],
    identifier_hash: str,
    connection: sqlite3.Connection,
) -> None:
    connection.executemany(
        f"""
        DELETE FROM {table_name}
        WHERE rowid = ?
        """,  # noqa: S608
        ((rowid,) for rowid in rowids),
    )
    connection.executemany(
        """
        DELETE FROM identifier_to_rowid
        WHERE hash = ?
        """,
        (identifier_hash,),
    )


def _save_identifier_to_rowid(
    rowids: Iterable[str], identifier_hash: str, connection: sqlite3.Connection
) -> None:
    connection.executemany(
        "INSERT INTO identifier_to_rowid(rowid, hash) VALUES (?,?)",
        ((rowid, identifier_hash) for rowid in rowids),
    )


def _create_tables_if_not_exists(connection: sqlite3.Connection) -> None:
    connection.execute(
        """
        CREATE TABLE IF NOT EXISTS identifier_to_rowid
        (
            rowid INTEGER,
            hash TEXT
        )
        """
    )


@contextmanager
def _transaction(connection: sqlite3.Connection) -> Iterator[None]:
    try:
        yield
        connection.commit()
    except Exception:
        log.exception("Transaction failed")
        connection.rollback()
