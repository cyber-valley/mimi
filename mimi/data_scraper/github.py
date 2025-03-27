import http.server
import json
import logging
import os
import subprocess
from abc import ABC, abstractmethod
from collections.abc import Iterable, Iterator
from contextlib import contextmanager
from dataclasses import dataclass, field
from datetime import UTC, datetime
from pathlib import Path
from typing import NoReturn

import requests
from result import Err, Ok, Result

from mimi import DataOrigin, DataSink

from . import DataScraperMessage

log = logging.getLogger(__name__)


class GithubScraperStopped(Exception):  # noqa: N818
    pass


@dataclass(frozen=True)
class GitRepository:
    owner: str
    name: str


@dataclass
class GithubScraperContext:
    port: int
    repository_base_path: Path
    repositories_to_follow: set[GitRepository]
    run_server: bool
    host: str = field(default="localhost")


def scrape(
    context: GithubScraperContext, sink: DataSink[DataScraperMessage]
) -> NoReturn:
    if not context.repository_base_path.exists():
        log.info("Creating repository base path")
        context.repository_base_path.mkdir(parents=True)

    log.info(
        "Starting syncing %s github repositories", len(context.repositories_to_follow)
    )
    with _set_directory(context.repository_base_path):
        for repository in context.repositories_to_follow:
            _scrape_git_repository(sink, repository)

    log.info("Starting downloading issues")
    for repository in context.repositories_to_follow:
        _scrape_issues(sink, repository)

    if not context.run_server:
        raise GithubScraperStopped

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


def _scrape_issues(
    sink: DataSink[DataScraperMessage], repository: GitRepository
) -> None:
    api_url = (
        f"https://api.github.com/repos/{repository.owner}/{repository.name}/issues"
    )

    page = 1
    while True:
        response = requests.get(
            api_url, params={"page": str(page), "state": "all"}, timeout=5
        )
        # FIXME: Wrap into own function with retry to handle rate limits
        response.raise_for_status()

        issues = response.json()
        if not issues:
            break

        log.info("Got %s issues from page %s", len(issues), page)
        for issue in issues:
            log.debug("Processing issue %s", issue)
            response = requests.get(issue["comments_url"], timeout=3)
            # FIXME: Wrap into own function with retry to handle rate limits
            response.raise_for_status()
            comments = response.json()

            title = issue["title"] or ""
            if not title:
                log.warning("Got issue with empty title")
            assignee_login = issue.get("assignee", {}).get("login")
            body = issue["body"] or ""
            if not body:
                log.warning("Got issue with empty body")
            comments = "\n".join(
                f"Comment from @{login}: {body}"
                for login, body in (
                    (comment["user"]["login"], comment.get("body", "empty comment"))
                    for comment in comments
                )
            )

            data = (
                title
                + "\nAssigned to: @"
                + assignee_login
                + "\n\n"
                + body
                + "\n\nComments:\n"
                + comments
            )
            sink.put(
                DataScraperMessage(
                    data=data,
                    origin=DataOrigin.GITHUB,
                    scraped_at=datetime.now(UTC),
                    pub_date=datetime.strptime(
                        issue["updated_at"], "%Y-%m-%dT%H:%M:%SZ"
                    ).replace(tzinfo=UTC),
                    identifier=f"{repository.owner}/{repository.name}@{issue['number']}",
                )
            )

        page += 1


def _scrape_git_repository(
    sink: DataSink[DataScraperMessage], repository: GitRepository
) -> None:
    repository_owner_path = Path(repository.owner)
    repository_owner_path.mkdir(exist_ok=True)
    with _set_directory(repository_owner_path):
        repository_path = Path(repository.name)
        if repository_path.exists():
            with _set_directory(repository_path):
                log.info("Updating %s", repository.name)
                _git("pull").unwrap()
                match _git("diff", "--name-only", "HEAD@{1}", "HEAD"):
                    case Ok(pulled_files):
                        _scrape_files(sink, repository, map(Path, pulled_files))
                    case Err(128):
                        log.info("Nothing to update")
                    case Err(unknown):
                        log.error("Got unknown diff return code %s", unknown)
        else:
            _git("clone", f"https://github.com/{repository.owner}/{repository.name}")
            with _set_directory(repository_path):
                _scrape_files(sink, repository, map(Path, _git("ls-files").unwrap()))


def _scrape_files(
    sink: DataSink[DataScraperMessage],
    repository: GitRepository,
    files: Iterable[Path],
) -> None:
    for file in files:
        if not file.exists():
            log.error("Profived file %s not found", file)
            continue
        try:
            data = file.read_text()
        except UnicodeDecodeError:
            log.warning("Failed to read text of %s in %s", file, repository)
            continue
        except IsADirectoryError:
            log.warning(
                "Got directory %s instead of file in %s",
                file,
                repository,
            )
            continue

        sink.put(
            DataScraperMessage(
                data=data,
                identifier=f"{repository.owner}/{repository.name}@{file}",
                origin=DataOrigin.GITHUB,
                scraped_at=datetime.now(UTC),
                pub_date=_get_last_commit_date(file),
            )
        )


def _get_last_commit_date(path: Path) -> datetime:
    iso_date, *_ = _git(
        "log", "-n", "1", "--format=%ad", "--date=iso", str(path)
    ).unwrap()
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
            return

        match self.headers.get("X-GitHub-Event"):
            case "push":
                log.info("Received push event. Payload=%s", payload)
                repository = GitRepository(*payload["full_name"].split("/"))
                if repository not in self._context.repositories_to_follow:
                    log.warning(
                        "Got push event for the unfollowed repository %s", repository
                    )
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


def _git(*args: str) -> Result[list[str], int]:
    command = ["/usr/bin/git", *args]
    try:
        result = subprocess.run(  # noqa: S603
            command,
            check=True,
            text=True,
            capture_output=True,
        )
    except subprocess.CalledProcessError as e:
        return Err(e.returncode)

    log.debug("Command [%s] finished", " ".join(command))
    return Ok(result.stdout.strip().split("\n"))


@contextmanager
def _set_directory(path: Path) -> Iterator[None]:
    origin = Path().absolute()
    try:
        os.chdir(path)
        log.debug("Switched current working directory to %s", path)
        yield
    except FileNotFoundError:
        log.exception("Current cwd: %s", origin)
        raise
    finally:
        os.chdir(origin)
        log.debug("Switched current working directory back to %s", path)
