from dataclasses import dataclass
from datetime import timedelta
from pathlib import Path
from typing import Any, Self

from mimi.data_scraper.github import GithubScraperContext, GitRepository
from mimi.data_scraper.telegram import PeersConfig, TelegramScraperContext
from mimi.data_scraper.x import XScraperContext
from mimi.domain import EmbeddingProvider, LLMProvider


@dataclass
class EmbeddingPipelineContext:
    db_file: Path
    embedding_provider: EmbeddingProvider
    embedding_model_name: str
    embedding_table_name: str

    @classmethod
    def from_dict(cls, d: dict[str, Any]) -> Self:
        d = d["embedding"]
        return cls(
            db_file=Path(d["db_file"]),
            embedding_provider=EmbeddingProvider(d["embedding_provider"]),
            embedding_model_name=d["embedding_model_name"],
            embedding_table_name=d["embedding_table_name"],
        )


@dataclass
class ScrapersContext:
    x: None | XScraperContext
    telegram: None | TelegramScraperContext
    github: None | GithubScraperContext

    @classmethod
    def from_dict(cls, d: dict[str, Any]) -> Self:
        d = d["scrapers"]
        github_repos = {
            GitRepository(**repo)
            for repo in d["github"].get("repositories_to_follow", [])
        }

        peers_config_data = d["telegram"].get(
            "peers_config", {"groups_ids": [], "fourms_ids": []}
        )
        peers_config = PeersConfig(**peers_config_data)

        x = XScraperContext(
            user_tweets_json_directory=Path(
                d["x"].get("user_tweets_json_directory", "")
            ),
            accounts_to_follow=d["x"].get("accounts_to_follow", []),
            poll_interval=_deserialize_timedelta(d["x"]["poll_interval"])
            if d["x"].get("poll_interval")
            else None,
        )

        telegram = TelegramScraperContext(
            peers_config=peers_config,
            history_depth=d["telegram"].get("history_depth", 50),
            process_new=d["telegram"].get("process_new", True),
        )

        github = GithubScraperContext(
            port=d["github"].get("port", 8000),
            host=d["github"].get("host", "localhost"),
            repository_base_path=Path(
                d["github"].get("repository_base_path", "github-repositories")
            ),
            repositories_to_follow=github_repos,
            run_server=d["github"].get("run_server", True),
        )

        assert any(item is not None for item in (x, telegram, github)), (
            "At least one scraper should be configured."
        )

        return cls(x=x, telegram=telegram, github=github)


@dataclass
class EmbeddingPipelineConfig:
    scrapers: ScrapersContext
    embedding: EmbeddingPipelineContext

    @classmethod
    def from_dict(cls, d: dict[str, Any]) -> Self:
        return cls(
            scrapers=ScrapersContext.from_dict(d),
            embedding=EmbeddingPipelineContext.from_dict(d),
        )


@dataclass
class LLMContext:
    provider: LLMProvider
    model: str

    @classmethod
    def from_dict(cls, d: dict[str, Any]) -> Self:
        return cls(
            provider=LLMProvider(d["llm"]["provider"]),
            model=d["llm"]["embedding_table_name"],
        )


@dataclass
class TelegramConfig:
    llm: LLMContext
    embedding: EmbeddingPipelineContext

    @classmethod
    def from_dict(cls, json_data: dict[str, Any]) -> Self:
        return cls(
            llm=LLMContext.from_dict(json_data),
            embedding=EmbeddingPipelineContext.from_dict(json_data),
        )


def _deserialize_timedelta(s: str) -> timedelta:
    match (int(s[:-1]), s[-1]):
        case [duration, "s"]:
            return timedelta(seconds=duration)
        case [duration, "m"]:
            return timedelta(minutes=duration)
        case [duration, "h"]:
            return timedelta(hours=duration)
        case [duration, "d"]:
            return timedelta(days=duration)
        case _:
            raise TimedeltaDeserializeError(s)


class TimedeltaDeserializeError(Exception):
    def __init__(self, value: str):
        super().__init__(f"Invalid duration format: {value}")
