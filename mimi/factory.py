import logging
import os
import sqlite3
from pathlib import Path
from typing import assert_never

import sqlite_vec
from langchain_community.vectorstores import VectorStore
from langchain_community.vectorstores.sqlitevec import SQLiteVec
from langchain_core.language_models.chat_models import BaseChatModel
from langchain_openai import ChatOpenAI, OpenAIEmbeddings

from .domain import EmbeddingProvider, LLMProvider
from .sqlite_extension import (
    TranscationlessConnectionProxy,
    create_connection_proxy,
    sqlite3_transaction,
)

log = logging.getLogger(__name__)


def get_connection(db_file: Path) -> sqlite3.Connection:
    connection = sqlite3.connect(db_file, autocommit=False, check_same_thread=False)
    connection.row_factory = sqlite3.Row
    connection.enable_load_extension(True)  # noqa: FBT003
    sqlite_vec.load(connection)
    connection.enable_load_extension(False)  # noqa: FBT003
    with sqlite3_transaction(connection):
        _create_tables_if_not_exists(connection)
    return connection


def get_vector_store(
    connection: sqlite3.Connection,
    embedding_provider: EmbeddingProvider,
    embedding_model_name: str,
    embedding_table_name: str,
) -> VectorStore:
    match embedding_provider:
        case EmbeddingProvider.OPENAI:
            embeddings = OpenAIEmbeddings(model=embedding_model_name)
            log.info("Using OpenAI embeddings %s", embedding_model_name)
        case _:
            assert_never(embedding_provider)
    vector_store = SQLiteVec(
        embedding_table_name,
        create_connection_proxy(connection, TranscationlessConnectionProxy),
        embeddings,
    )
    with sqlite3_transaction(connection):
        vector_store.create_table_if_not_exists()
    return vector_store


def get_llm(provider: LLMProvider, model: str) -> BaseChatModel:
    match provider:
        case LLMProvider.OPENROUTER:
            return ChatOpenAI(
                api_key=os.environ["OPENROUTER_API_KEY"],  # type: ignore[arg-type]
                base_url=os.environ["OPENROUTER_API_URL"],
                model=model,
            )
        case LLMProvider.OPENAI:
            return ChatOpenAI(
                model=model,
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
