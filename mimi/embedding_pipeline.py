import hashlib
import logging
import sqlite3
from collections.abc import Iterable
from dataclasses import dataclass, field
from datetime import UTC, datetime
from enum import Enum, auto
from typing import NoReturn, assert_never

import tenacity
from langchain_community.vectorstores.sqlitevec import SQLiteVec
from langchain_openai import OpenAIEmbeddings
from langchain_text_splitters import RecursiveCharacterTextSplitter

from .data_scraper import DataScraperMessage
from .data_sink import DataSink

log = logging.getLogger(__name__)


class EmbeddingType(Enum):
    OPENAI = auto()


@dataclass
class EmbeddingPipelineContext:
    db_file: str
    embedding_type: EmbeddingType
    embedding_model_name: str
    vector_dimension: int = field(default=1536)
    delta_threshold: float = field(default=1.0)


# TODO @inspektorkek handle embedding chunk size
@tenacity.retry(
    retry=tenacity.retry_if_exception_type(sqlite3.OperationalError),
    wait=tenacity.wait_exponential(multiplier=1, max=10),
)
def run_embedding_pipeline(
    context: EmbeddingPipelineContext, sink: DataSink[DataScraperMessage]
) -> NoReturn:
    """
    Listens to updates from the `sink` and for each:
    - Saves raw data into the database
    - Calculates and saves embedding
    - Checks delta between now and `scraped_at` to track late delivery
    """
    embedding_table_name: str = "embedding"

    match context.embedding_type:
        case EmbeddingType.OPENAI:
            embeddings = OpenAIEmbeddings(model=context.embedding_model_name)
        case _:
            assert_never(context.embedding_type)

    text_splitter = RecursiveCharacterTextSplitter(chunk_size=1000, chunk_overlap=200)
    connection = SQLiteVec.create_connection(db_file=context.db_file)
    _create_tables_if_not_exists(connection)
    vectorstore = SQLiteVec(embedding_table_name, connection, embeddings)

    processed_count = 0
    while True:
        message = sink.get()
        metadata = {
            "origin": message.origin,
            "scraped_at": int(message.scraped_at.replace(tzinfo=UTC).timestamp()),
            "pub_date": int(message.pub_date.replace(tzinfo=UTC).timestamp()),
        }
        splits = text_splitter.split_text(message.data)
        identifier_hash = hashlib.sha256(message.identifier.encode()).digest()
        log.debug("Processing message from %s", message.origin)

        rowids = _find_rowids(identifier_hash, connection)
        if rowids:
            _delete_embedding(embedding_table_name, rowids, identifier_hash, connection)
            log.debug("Deleted embedding for message")

        rowids = vectorstore.add_texts(
            texts=splits,
            metadatas=[metadata for _ in splits],
        )
        _save_identifier_to_rowid(rowids, identifier_hash, connection)
        log.debug("Saved embedding for message")

        now = datetime.now(UTC)
        delay = (now - message.scraped_at).total_seconds()
        log.debug("Message delivered with %.2fs delay", delay)

        if message.scraped_at < message.pub_date:
            log.error(
                "Scraped time (%.2fs) is earlier than publication time (%.2fs)!",
                message.scraped_at.timestamp(),
                message.pub_date.timestamp(),
            )

        log.error(
            "Delay %.2fs is less than threshold %.2fs!", delay, context.delta_threshold
        ) if delay < context.delta_threshold else None

        processed_count += 1
        if processed_count % 100 == 0:
            log.info(
                "Processed %d messages. Last message delivered with %.2fs delay",
                processed_count,
                delay,
            )
            processed_count = 0


def _find_rowids(identifier_hash: bytes, connection: sqlite3.Connection) -> list[str]:
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


def _delete_embedding(
    table_name: str,
    rowids: Iterable[str],
    identifier_hash: bytes,
    connection: sqlite3.Connection,
) -> None:
    connection.executemany(
        f"""
            DELETE FROM {table_name}
            WHERE rowid = ?;
        """,  # noqa: S608
        ((rowid,) for rowid in rowids),
    )
    connection.executemany(
        """
            DELETE FROM identifier_to_rowid
            WHERE hash = ?;
        """,
        (identifier_hash,),
    )

    connection.commit()


def _save_identifier_to_rowid(
    rowids: Iterable[str], identifier_hash: bytes, connection: sqlite3.Connection
) -> None:
    connection.executemany(
        "INSERT INTO identifier_to_rowid(rowid, hash) VALUES (?,?)",
        ((rowid, identifier_hash) for rowid in rowids),
    )
    connection.commit()


def _create_tables_if_not_exists(connection: sqlite3.Connection) -> None:
    connection.execute(
        """
        CREATE TABLE IF NOT EXISTS identifier_to_rowid
        (
            rowid INTEGER,
            hash BLOB PRIMARY KEY
        )
        ;
        """
    )
    connection.commit()
