import hashlib
import logging
import sqlite3
from dataclasses import dataclass, field
from datetime import UTC, datetime
from enum import Enum, auto
from typing import NoReturn, assert_never

import tenacity
from langchain_community.vectorstores.sqlitevec import SQLiteVec, serialize_f32
from langchain_openai import OpenAIEmbeddings

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

    connection = SQLiteVec.create_connection(db_file=context.db_file)
    _create_tables_if_not_exists(connection=connection)
    vectorstore = SQLiteVec(
        table=embedding_table_name, connection=connection, embedding=embeddings
    )

    processed_count = 0
    while True:
        message = sink.get()
        identifier_hash = hashlib.sha256(message.data.encode()).digest()
        log.debug("Processing message from %s", message.origin)

        rowid = _find_rowid(identifier_hash=identifier_hash, connection=connection)
        if rowid is None:
            rowid, *_ = vectorstore.add_texts(
                texts=[message.data],
                metadatas=[
                    {
                        "origin": message.origin,
                        "scraped_at": int(
                            message.scraped_at.replace(tzinfo=UTC).timestamp()
                        ),
                        "pub_date": int(
                            message.pub_date.replace(tzinfo=UTC).timestamp()
                        ),
                    }
                ],
            )
            _save_identifier_to_rowid(
                rowid=rowid, identifier_hash=identifier_hash, connection=connection
            )

            log.debug("Saved embedding for message")
        else:
            text_embedding = embeddings.embed_documents([message.data])
            assert len(text_embedding) == 1, f"{len(text_embedding)=}"
            _update_embedding(
                embedding_table_name, rowid, message.data, text_embedding[0], connection
            )

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


def _find_rowid(identifier_hash: bytes, connection: sqlite3.Connection) -> str | None:
    row = connection.execute(
        """
        SELECT rowid FROM identifier_to_rowid
        WHERE hash = ?
        """,
        (identifier_hash,),
    ).fetchone()

    if row is None:
        return None

    return str(row[0])


def _update_embedding(
    table_name: str,
    rowid: str,
    text: str,
    text_embedding: list[float],
    connection: sqlite3.Connection,
) -> None:
    connection.execute(
        f"""
            UPDATE {table_name} SET text = ?, text_embedding = ?
            WHERE rowid = ?
        """,
        (text, serialize_f32(text_embedding), rowid),
    )
    connection.commit()


def _save_identifier_to_rowid(
    rowid: str, identifier_hash: bytes, connection: sqlite3.Connection
) -> None:
    connection.execute(
        "INSERT INTO identifier_to_rowid(rowid, hash) VALUES (?,?)",
        (rowid, identifier_hash),
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
