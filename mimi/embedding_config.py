from dataclasses import dataclass
from datetime import timedelta
from pathlib import Path
from typing import Any, Self

from mimi.data_scraper.github import GithubScraperContext, GitRepository
from mimi.data_scraper.telegram import PeersConfig, TelegramScraperContext
from mimi.data_scraper.x import XScraperContext
from mimi.embedding_pipeline import EmbeddingPipelineContext, EmbeddingType


@dataclass
class ScrapersContext:
    x: None | XScraperContext
    telegram: None | TelegramScraperContext
    github: None | GithubScraperContext


@dataclass
class EmbeddingPipelineConfig:
    scrapers: ScrapersContext
    embedding: EmbeddingPipelineContext

    @classmethod
    def from_dict(cls, json_data: dict[str, Any]) -> Self:
        github_repos = {
            GitRepository(**repo)
            for repo in json_data["scrapers"]["github"].get(
                "repositories_to_follow", []
            )
        }

        peers_config_data = json_data["scrapers"]["telegram"].get(
            "peers_config", {"groups_ids": [], "fourms_ids": []}
        )
        peers_config = PeersConfig(**peers_config_data)

        x_context = XScraperContext(
            user_tweets_json_directory=Path(
                json_data["scrapers"]["x"].get("user_tweets_json_directory", "")
            ),
            accounts_to_follow=json_data["scrapers"]["x"].get("accounts_to_follow", []),
            poll_interval=_deserialize_timedelta(
                json_data["scrapers"]["x"]["poll_interval"]
            )
            if json_data["scrapers"]["x"].get("poll_interval")
            else None,
        )

        telegram_context = TelegramScraperContext(
            peers_config=peers_config,
            history_depth=json_data["scrapers"]["telegram"].get("history_depth", 50),
            process_new=json_data["scrapers"]["telegram"].get("process_new", True),
        )

        github_context = GithubScraperContext(
            port=json_data["scrapers"]["github"].get("port", 8000),
            host=json_data["scrapers"]["github"].get("host", "localhost"),
            repository_base_path=Path(
                json_data["scrapers"]["github"].get(
                    "repository_base_path", "github-repositories"
                )
            ),
            repositories_to_follow=github_repos,
            run_server=json_data["scrapers"]["github"].get("run_server", True),
        )

        scrapers_context = ScrapersContext(
            x=x_context, telegram=telegram_context, github=github_context
        )

        embedding_context = EmbeddingPipelineContext(
            db_file=Path(json_data["embedding"]["db_file"]),
            embedding_type=EmbeddingType(json_data["embedding"]["embedding_type"]),
            embedding_model_name=json_data["embedding"]["embedding_model_name"],
        )

        return cls(scrapers=scrapers_context, embedding=embedding_context)


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
