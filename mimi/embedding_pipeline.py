import logging
import sqlite3
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import NoReturn

from langchain.embeddings.openai import OpenAIEmbeddings
from langchain_community.vectorstores.sqlitevec import SQLiteVec

from .data_scraper import DataScraperMessage
from .data_sink import DataSink

log = logging.getLogger(__name__)


@dataclass
class EmbeddingPipelineContext:
    """Configuration for the embedding pipeline."""

    db_path: Path
    embedding_model: str = "text-embedding-3-small"
    vector_dimension: int = 1536


class EmbeddingPipeline:
    """Handles embedding generation and storage for messages."""

    def __init__(self, context: EmbeddingPipelineContext):
        self.context = context
        self.processed_count = 0
        self.embeddings = OpenAIEmbeddings(model=context.embedding_model)

        try:
            self.conn = sqlite3.connect(str(context.db_path))
            self.vectorstore = SQLiteVec(
                table="message_embeddings",
                embedding=self.embeddings,
                connection=self.conn,
            )
            self.vectorstore.create_table_if_not_exists()
            log.info("Connected to SQLiteVec database at %s", context.db_path)
        except Exception:
            log.exception("Failed to initialize SQLiteVec at %s", context.db_path)
            raise

    def process_message(self, message: DataScraperMessage) -> None:
        """Process a single message: generate embedding and store it."""

        try:
            log.debug("Processing message from %s", message.origin)

            self.vectorstore.add_texts(
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

            now = datetime.now(UTC)
            delay = now - message.scraped_at
            self.processed_count += 1
            if self.processed_count % 100 == 0:
                log.info(
                    "Processed %d messages. Last message delivered with %.2fs delay",
                    self.processed_count,
                    delay.total_seconds(),
                )
            else:
                log.debug("Message delivered with %.2fs delay", delay.total_seconds())

        except Exception:
            log.exception("Unexpected error processing message")


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
    pipeline = EmbeddingPipeline(context)

    while True:
        try:
            message = sink.get()
            pipeline.process_message(message)
        except Exception:
            log.exception("Error in pipeline loop")
            continue
