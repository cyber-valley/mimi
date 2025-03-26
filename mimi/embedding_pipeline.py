import hashlib
import logging
import sqlite3
from collections.abc import Iterable
from datetime import UTC
from typing import NoReturn

import tenacity
from langchain_community.vectorstores import VectorStore
from langchain_text_splitters import RecursiveCharacterTextSplitter, TextSplitter

from .data_scraper import DataScraperMessage
from .data_sink import DataSink
from .sqlite_extension import sqlite3_transaction

log = logging.getLogger(__name__)


@tenacity.retry(
    retry=tenacity.retry_if_not_exception_type(sqlite3.OperationalError),
    wait=tenacity.wait_exponential(multiplier=1, max=10),
    before_sleep=tenacity.before_sleep_log(log, logging.ERROR, exc_info=True),
    after=tenacity.after_log(log, logging.INFO),
)
def run(
    sink: DataSink[DataScraperMessage],
    connection: sqlite3.Connection,
    vector_store: VectorStore,
    embedding_table_name: str,
) -> NoReturn:
    """
    Listens to updates from the `sink` and for each:
    - Saves raw data into the database
    - Calculates and saves embedding
    """
    log.info("Starting embedding pipeline")
    text_splitter = RecursiveCharacterTextSplitter(chunk_size=1000, chunk_overlap=200)

    while True:
        log.debug("Waiting for the new message")
        message = sink.get()
        _process_message(
            text_splitter,
            connection,
            vector_store,
            embedding_table_name,
            message,
        )


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
    vector_store: VectorStore,
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
    with sqlite3_transaction(connection):
        if rowids:
            _delete_embeddings(
                embedding_table_name, rowids, identifier_hash, connection
            )
            log.info("Deleted %s embeddings", len(rowids))

        rowids = vector_store.add_texts(
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
