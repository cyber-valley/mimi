import http.server
import json
import logging
import os
import subprocess
from abc import ABC, abstractmethod
from collections.abc import Collection, Iterator
from contextlib import contextmanager
from dataclasses import dataclass, field
from pathlib import Path
from typing import NoReturn

from mimi import DataSink

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
    repositories_to_follow: set[GithubRepository]
    host: str = field(default="localhost")


def scrape(
    context: GithubScraperContext, _sink: DataSink[DataScraperMessage]
) -> NoReturn:
    if not context.repository_base_path.exists():
        log.info("Creating repository base path")
        context.repository_base_path.mkdir(parents=True)

    log.info("Syncying git repositories")
    with _set_directory(context.repository_base_path):
        _sync_github_repositories(context.repositories_to_follow)

    # This is required for the context injection
    # to the handler
    class GithubWebhookHandler(BaseGithubWebhookHandler):
        @property
        def _context(self) -> GithubScraperContext:
            return context

    server_address = (context.host, context.port)
    httpd = http.server.HTTPServer(server_address, GithubWebhookHandler)
    log.info("Starting github webhook server on %s:%s", context.host, context.port)
    httpd.serve_forever()
    raise GithubScraperStopped


def _sync_github_repositories(
    repositories_to_follow: Collection[GithubRepository],
) -> None:
    for repository in repositories_to_follow:
        repository_owner_path = Path(repository.owner)
        repository_owner_path.mkdir(exist_ok=True)
        repository_path = repository_owner_path / repository.name
        if repository_path.exists():
            with _set_directory(repository_path):
                _git_pull()
                # TODO: Sync only pulled files
                _sync_project_files()
        else:
            with _set_directory(repository_owner_path):
                _git_clone(f"https://github.com/{repository.owner}/{repository.name}")
                with _set_directory(repository_path):
                    _sync_project_files()


def _sync_project_files() -> None:
    pass


class BaseGithubWebhookHandler(http.server.BaseHTTPRequestHandler, ABC):
    @property
    @abstractmethod
    def _context(self) -> GithubScraperContext: ...

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
                raise NotImplementedError
            case "issues":
                log.info("Received issues event. Payload=%s", payload)
                raise NotImplementedError
            case "pull_request":
                log.info("Received pull_request event. Payload=%s", payload)
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


def _git_clone(url: str) -> None:
    subprocess.run(["/usr/bin/git", "clone", url], check=True, capture_output=False)  # noqa: S603
    log.info("[git clone %s] finished")


def _git_pull() -> None:
    subprocess.run(  # noqa: S603
        ["/usr/bin/git", "pull"],
        check=True,
        capture_output=True,
    )
    log.info("[git pull] finished")


@contextmanager
def _set_directory(path: Path) -> Iterator[None]:
    origin = Path().absolute()
    try:
        os.chdir(path)
        yield
    finally:
        os.chdir(origin)
