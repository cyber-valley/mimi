import http.server
import json
import logging
import os
import subprocess
from abc import ABC, abstractmethod
from collections.abc import Collection, Iterator
from contextlib import contextmanager
from dataclasses import dataclass, field
from datetime import UTC, datetime
from pathlib import Path
from typing import NoReturn

from mimi import DataOrigin, DataSink

from . import DataScraperMessage

log = logging.getLogger(__name__)


class GithubScraperStopped(Exception):  # noqa: N818
    pass


@dataclass
class GithubRepository:
    owner: str
    name: str


@dataclass
class GithubScraperContext:
    port: int
    repository_base_path: Path
    repositories_to_follow: set[GitRepository]
    host: str = field(default="localhost")


def scrape(
    context: GithubScraperContext, sink: DataSink[DataScraperMessage]
) -> NoReturn:
    if not context.repository_base_path.exists():
        log.info("Creating repository base path")
        context.repository_base_path.mkdir(parents=True)

    with _set_directory(context.repository_base_path):
        _sync_github_repositories(sink, context.repositories_to_follow)

    # This is required for the context injection
    # to the handler
    class GithubWebhookHandler(_BaseGithubWebhookHandler):
        @property
        def _context(self) -> GithubScraperContext:
            return context

        @property
        def _sink(self) -> DataSink[DataScraperMessage]:
            return sink


    server_address = (context.host, context.port)
    httpd = http.server.HTTPServer(server_address, GithubWebhookHandler)
    log.info("Starting github webhook server on %s:%s", context.host, context.port)
    httpd.serve_forever()
    raise GithubScraperStopped


def _sync_github_repositories(
    sink: DataSink[DataScraperMessage],
    repositories_to_follow: Collection[GitRepository],
) -> None:
    log.info("Starting syncing %s github repositories", len(repositories_to_follow))
    for repository in repositories_to_follow:
        _scrape_git_repository(sink, repository)


def _scrape_git_repository(
    sink: DataSink[DataScraperMessage],
    repository: GithubRepository
) -> None:
    repository_owner_path = Path(repository.owner)
    repository_owner_path.mkdir(exist_ok=True)
    repository_path = repository_owner_path / repository.name
    if repository_path.exists():
        with _set_directory(repository_path):
            _git("pull")
            pulled_files = [
                Path(file)
                for file in _git(
                    "diff", "--name-only", "--diff-filter=AM", "HEAD@{1}", "HEAD"
                )
            ]
            _scrape_files(sink, pulled_files)
    else:
        with _set_directory(repository_owner_path):
            _git(
                "clone", f"https://github.com/{repository.owner}/{repository.name}"
            )
            repository_files = [Path(file) for file in _git("ls-files")]
            with _set_directory(repository_path):
                _scrape_files(sink, repository_files)


def _scrape_files(sink: DataSink[DataScraperMessage], files: Collection[Path]) -> None:
    for file in files:
        if not file.exists():
            log.error("Profived file %s not found", file)
            continue
        sink.put(
            DataScraperMessage(
                data=file.read_text(),
                origin=DataOrigin.GITHUB,
                scraped_at=datetime.now(UTC),
                pub_date=_get_last_commit_date(file),
            )
        )


def _get_last_commit_date(path: Path) -> datetime:
    iso_date, *_ = _git("log", "-n", "1", "--format=%ad", "--date=iso", str(path))
    return datetime.fromisoformat(iso_date)


class _BaseGithubWebhookHandler(http.server.BaseHTTPRequestHandler, ABC):
    @property
    @abstractmethod
    def _context(self) -> GithubScraperContext: ...


    @property
    @abstractmethod
    def _sink(self) -> DataSink[DataScraperMessage]: ...


    def do_POST(self) -> None:  # noqa: N802
        if self.path == "/webhook":
            self._process_webhook()
        else:
            self.send_response(404)
            self.send_header("Content-type", "application/json")
            self.end_headers()

    def _process_webhook(self) -> None:
        content_length = int(self.headers["Content-Length"])
        post_data = self.rfile.read(content_length)

        try:
            payload = json.loads(post_data.decode("utf-8"))
        except json.JSONDecodeError:
            self.send_response(400)
            self.send_header("Content-type", "application/json")
            self.end_headers()
            self.wfile.write(
                json.dumps(
                    {"status": "Error", "message": "Invalid JSON payload"}
                ).encode("utf-8")
            )
            return

        match self.headers.get("X-GitHub-Event"):
            case "push":
                log.info("Received push event. Payload=%s", payload)
                repository = GitRepository(payload["full_name"].split("/"))
                if repository not in self._content.repositories_to_follow:
                    log.warning("Got push event for the unfollowed repository %s", repository)
                with _set_directory(self._context.repository_base_path):
                    _scrape_git_repository(self._sink, repository)
            case "issues":
                log.info("Received issues event. Payload=%s", payload)
                raise NotImplementedError
            case event_type:
                log.warning(
                    "Received unknown event_type=%s with payload=%s",
                    event_type,
                    payload,
                )

        self.send_response(200)
        self.send_header("Content-type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps({"status": "OK"}).encode("utf-8"))


def _git(*args: str) -> list[str]:
    result = subprocess.run(  # noqa: S603
        ["/usr/bin/git", *args],
        check=True,
        text=True,
        capture_output=True,
    )
    log.info("Command [git %s] finished", " ".join(args))
    return result.stdout.strip().split("\n")


@contextmanager
def _set_directory(path: Path) -> Iterator[None]:
    origin = Path().absolute()
    try:
        os.chdir(path)
        yield
    finally:
        os.chdir(origin)
