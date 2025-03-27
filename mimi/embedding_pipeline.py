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
    log.debug("[%s] Processing message from %s", identifier_hash, message.origin)

    rowids_to_texts = _find_rowids_to_texts_by_identifer_hash(
        identifier_hash, connection, embedding_table_name
    )
    data_hash = hashlib.sha256(message.data.encode()).digest()
    updated_rowids = [
        rowid
        for rowid, text in rowids_to_texts.items()
        if hashlib.sha256(message.data.encode()).digest() != data_hash
    ]

    with sqlite3_transaction(connection):
        if updated_rowids:
            _delete_embeddings(
                embedding_table_name, updated_rowids, identifier_hash, connection
            )
            log.info("[%s] Deleted %s embeddings", identifier_hash, len(updated_rowids))

        if not rowids_to_texts or updated_rowids:
            rowids = vector_store.add_texts(
                texts=splits,
                metadatas=[metadata for _ in splits],
            )
            _save_identifier_to_rowid(rowids, identifier_hash, connection)
            log.info("[%s] Saved %s embeddings", identifier_hash, len(rowids))
        else:
            log.info("[%s] Has actual embedding already", identifier_hash)


def _find_rowids_to_texts_by_identifer_hash(
    identifier_hash: str, connection: sqlite3.Connection, embedding_table_name: str
) -> dict[str, str]:
    return {
        row["rowid"]: row["text"]
        for row in connection.execute(
            f"""
            SELECT itr.rowid, e.text FROM identifier_to_rowid itr
            INNER JOIN {embedding_table_name} e
              ON e.rowid = itr.rowid
            WHERE hash = ?
            """,  # noqa: S608
            (identifier_hash,),
        )
    }


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
    connection.execute(
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
