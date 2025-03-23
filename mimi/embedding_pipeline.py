import logging
import sqlite3
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import NoReturn

import tenacity
from langchain_community.embeddings import OpenAIEmbeddings
from langchain_community.vectorstores.sqlitevec import SQLiteVec

from .data_scraper import DataScraperMessage
from .data_sink import DataSink

log = logging.getLogger(__name__)


@dataclass
class EmbeddingPipelineContext:
    db_path: Path
    # XXX: Dataclass values should be configured with `field` function
    # FIXME: Remove hardcoded values, they should be setted on the
    # consturctor's coller side
    embedding_model: str = "text-embedding-3-small"
    vector_dimension: int = 1536

# FIXME: Move retry to the entry point, so all possible errors will be retried
# FIXME: Remove stop argument
@tenacity.retry(
    retry=tenacity.retry_if_exception_type(
        (sqlite3.OperationalError, sqlite3.DatabaseError)
    ),
    wait=tenacity.wait_exponential(multiplier=1, max=10),
    stop=tenacity.stop_after_attempt(5),
)
def create_vectorstore(context: EmbeddingPipelineContext) -> SQLiteVec:
    # FIXME: Provide embeddings model type in context as well
    embeddings = OpenAIEmbeddings(model=context.embedding_model)

    # XXX: Why do we need `check_same_thread`
    # Looks like given connection will be used only in one thread
    # FIXME: Needless cast to `str`, `pathlib.Path` implements `os.PathLike`
    # FIXME: Use `SQLiteVec.create_connection` instead
    conn = sqlite3.connect(str(context.db_path), check_same_thread=False)

    # FIXME: Remove `message_` prefix in the table's name
    # We embed message's data, not message itself
    vectorstore = SQLiteVec(
        table="message_embeddings",
        embedding=embeddings,
        connection=conn,
    )
    vectorstore.create_table_if_not_exists()

    log.info("Connected to SQLiteVec database at %s", context.db_path)
    return vectorstore


def run_embedding_pipeline(
    context: EmbeddingPipelineContext, sink: DataSink[DataScraperMessage]
) -> NoReturn:
    """
    Listens to updates from the `sink` and for each:
    - Saves raw data into the database
    - Calculates and saves embedding
    - Checks delta between now and `scraped_at` to track late delivery
    """
    log.info("Starting embedding pipeline with model %s", context.embedding_model)
    vectorstore = create_vectorstore(context)

    # FIXME: Move delta to context
    delta_threshold = 1.0
    processed_count = 0

    while True:
        message = sink.get()
        log.debug("Processing message from %s", message.origin)

        # FIXME: Remove general use try expect, retry on the function will handle
        try:
            # TODO: Store sha256 hash of `DataScraperMessage.identifier` (introduced in c6ca26e@main)
            # To make it working addition field should be added to the table
            # We don't want to use metadata because it's difficult to query it
            # FIXME: It's better to store datetimes as UNIX timestampt
            # without floating point part
            vectorstore.add_texts(
                texts=[message.data],
                metadatas=[
                    {
                        "origin": message.origin,
                        "scraped_at": message.scraped_at.isoformat(),
                        "pub_date": message.pub_date.isoformat(),
                    }
                ],
            )
            log.debug("Saved embedding for message")

            # TODO: Check time between `scraped_at` and `pub_date` as well
            now = datetime.now(UTC)
            delay = (now - message.scraped_at).total_seconds()
            log.debug("Message delivered with %.2fs delay", delay)

            # TODO: Let's use `log.error` instead of assert
            # Otherwise we will restart and loose time on it
            assert delay > delta_threshold, (
                f"Delay {delay:.2f}s is less than threshold {delta_threshold:.2f}s!"
            )

            processed_count += 1
            # FIXME: Set counter to zero on if statement
            # We don't want a big ass integer in the memory
            if processed_count % 100 == 0:
                log.info(
                    "Processed %d messages. Last message delivered with %.2fs delay",
                    processed_count,
                    delay,
                )

        except Exception as e:
            log.exception("Unexpected error: %s", e, exc_info=True)
            raise
