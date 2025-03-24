import hashlib
import logging
import sqlite3
from collections.abc import Iterable
from dataclasses import dataclass
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


@tenacity.retry(
    retry=tenacity.retry_if_exception_type(_should_not_be_raised),
    wait=tenacity.wait_exponential(multiplier=1, max=10),
)
def run_embedding_pipeline(
    context: EmbeddingPipelineContext, sink: DataSink[DataScraperMessage]
) -> NoReturn:
    """
    Listens to updates from the `sink` and for each:
    - Saves raw data into the database
    - Calculates and saves embedding
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
            _delete_embeddings(embedding_table_name, rowids, identifier_hash, connection)
            log.info("Deleted %s embeddings", len(rowids))

        rowids = vectorstore.add_texts(
            texts=splits,
            metadatas=[metadata for _ in splits],
        )
        _save_identifier_to_rowid(rowids, identifier_hash, connection)
        log.debug("Saved %s embeddings", len(rowids))


def _should_not_be_raised(exception: Exception) -> bool:
    return not isinstance(exception, sqlite3.OperationalError)


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


def _delete_embeddings(
    table_name: str,
    rowids: Iterable[str],
    identifier_hash: bytes,
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
        """
    )
    connection.commit()
