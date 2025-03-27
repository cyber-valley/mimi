import hashlib
import hmac
import http.server
import json
import logging
import os
import subprocess
from abc import ABC, abstractmethod
from collections.abc import Iterable, Iterator, Mapping
from contextlib import contextmanager
from dataclasses import dataclass, field
from datetime import UTC, datetime
from pathlib import Path
from typing import Any, NoReturn, Self, assert_never

import requests
import tenacity
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

    @classmethod
    def from_github_full_name(cls, full_name: str) -> Self:
        return cls(*full_name.split("/"))


@dataclass
class GithubScraperContext:
    port: int
    repository_base_path: Path
    repositories_to_follow: set[GitRepository]
    run_server: bool
    host: str = field(default="localhost")
    secret: str = field(default_factory=lambda: os.environ["GITHUB_WEBHOOK_SECRET"])
    personal_access_token: str = field(
        default_factory=lambda: os.environ["GITHUB_PERSONAL_ACCESS_TOKEN"]
    )


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
        _scrape_issues(sink, repository, context.personal_access_token)

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


@dataclass
class GithubIssueComment:
    user_login: str
    comment: str


@dataclass
class GithubIssue:
    number: int
    title: str
    assignee_login: None | str
    body: None | str
    comments: list[GithubIssueComment]
    updated_at: datetime


def _scrape_issues(
    sink: DataSink[DataScraperMessage],
    repository: GitRepository,
    personal_access_token: str,
    *,
    issues_urls: None | list[str] = None,
) -> None:
    issues: Iterable[GithubIssue]
    if issues_urls:
        issues = (_scrape_issue(personal_access_token, url) for url in issues_urls)
    else:
        issues = _scrape_all_issues(personal_access_token, repository)

    repository_url = "https://github.com/" + repository.owner + "/" + repository.name
    for issue in issues:
        data = (
            "GitHub Issue in repository "
            + repository_url
            + "\n title: "
            + issue.title
            + "\n url: "
            + repository_url
            + "/issues"
            + str(issue.number)
            + (
                ("\nAssigned to: @" + issue.assignee_login)
                if issue.assignee_login
                else "\nNot assigned yet"
            )
            + "\n\n"
            + (issue.body or "")
            + "\n\nComments:\n"
            + "\n".join(
                f"From @{comment.user_login}: {comment.comment}"
                for comment in issue.comments
            )
        )
        sink.put(
            DataScraperMessage(
                data=data,
                origin=DataOrigin.GITHUB,
                scraped_at=datetime.now(UTC),
                pub_date=issue.updated_at,
                identifier=f"{repository.owner}/{repository.name}@{issue.number}",
            )
        )


def _scrape_all_issues(
    personal_access_token: str, repository: GitRepository
) -> Iterable[GithubIssue]:
    url = f"https://api.github.com/repos/{repository.owner}/{repository.name}/issues"

    page = 1
    while True:
        issues = _scrape_issues_page(personal_access_token, url, page)
        if not issues:
            break
        yield from issues
        page += 1


@tenacity.retry(
    retry=tenacity.retry_if_exception_type(requests.HTTPError)
    | tenacity.retry_if_exception_type(requests.Timeout),
    wait=tenacity.wait_exponential(multiplier=1, max=10),
    stop=tenacity.stop_after_attempt(3),
    before_sleep=tenacity.before_sleep_log(log, logging.ERROR, exc_info=True),
    after=tenacity.after_log(log, logging.INFO),
)
def _scrape_issues_page(
    personal_access_token: str, url: str, page: int
) -> list[GithubIssue]:
    response = requests.get(
        url,
        headers={"Authorization": f"Bearer {personal_access_token}"},
        params={"page": str(page), "state": "all"},
        timeout=5,
    )
    response.raise_for_status()

    issues = response.json()
    if not issues:
        return []

    log.info("Got %s issues from page %s", len(issues), page)
    return [_scrape_issue(personal_access_token, issue) for issue in issues]


@tenacity.retry(
    retry=tenacity.retry_if_exception_type(requests.HTTPError)
    | tenacity.retry_if_exception_type(requests.Timeout),
    wait=tenacity.wait_exponential(multiplier=1, max=10),
    stop=tenacity.stop_after_attempt(3),
    before_sleep=tenacity.before_sleep_log(log, logging.ERROR, exc_info=True),
    after=tenacity.after_log(log, logging.INFO),
)
def _scrape_issue(personal_access_token: str, url_or_data: str | Any) -> GithubIssue:
    match url_or_data:
        case Mapping():
            issue = url_or_data
            log.debug("Processing issue %s", issue)

            title = issue["title"]
            assignee_login = issue.get("assignee", {}).get("login")
            body = issue["body"]
            if not body:
                log.warning("Got issue with empty body")

            return GithubIssue(
                number=issue["number"],
                title=title,
                assignee_login=assignee_login,
                body=body,
                comments=_scrape_issue_comments(
                    personal_access_token, issue["comments_url"]
                ),
                updated_at=datetime.strptime(
                    issue["updated_at"], "%Y-%m-%dT%H:%M:%SZ"
                ).replace(tzinfo=UTC),
            )
        case str():
            response = requests.get(
                url_or_data,
                headers={"Authorization": f"Bearer {personal_access_token}"},
                timeout=5,
            )
            response.raise_for_status()
            return _scrape_issue(personal_access_token, response.json())
        case _:
            assert_never(url_or_data)


@tenacity.retry(
    retry=tenacity.retry_if_exception_type(requests.HTTPError)
    | tenacity.retry_if_exception_type(requests.Timeout),
    wait=tenacity.wait_exponential(multiplier=1, max=10),
    stop=tenacity.stop_after_attempt(3),
    before_sleep=tenacity.before_sleep_log(log, logging.ERROR, exc_info=True),
    after=tenacity.after_log(log, logging.INFO),
)
def _scrape_issue_comments(
    personal_access_token: str, url: str
) -> list[GithubIssueComment]:
    log.debug("Scraping comments from %s", url)
    response = requests.get(
        url, headers={"Authorization": f"Bearer {personal_access_token}"}, timeout=3
    )
    response.raise_for_status()
    comments = response.json()
    log.info("Scraped %s comments", len(comments))
    return [
        GithubIssueComment(
            user_login=comment["user"]["login"],
            comment=comment.get("body", "empty comment"),
        )
        for comment in comments
    ]


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
        signature = self.headers.get("X-Hub-Signature-256")

        content_length = int(self.headers["Content-Length"])
        body = self.rfile.read(content_length)

        if not (
            signature and _validate_signature(body, signature, self._context.secret)
        ):
            self.send_response(403)
            self.end_headers()
            return

        try:
            payload = json.loads(body.decode("utf-8"))
        except json.JSONDecodeError:
            log.exception("Failed to parse payload of %s", body)
            self.send_response(400)
            return

        match [self.headers.get("X-GitHub-Event"), payload]:
            case ["push", {"repository": {"full_name": full_name}}]:
                log.info("Received push event. Payload=%s", payload)
                repository = GitRepository.from_github_full_name(full_name)
                if repository not in self._context.repositories_to_follow:
                    log.warning(
                        "Got push event for the unfollowed repository %s", repository
                    )
                with _set_directory(self._context.repository_base_path):
                    _scrape_git_repository(self._sink, repository)
            case [
                "issues" | "issue_comment",
                {"repository": {"full_name": full_name}, "issue": {"url": url}},
            ]:
                log.info("Received issues event. Payload=%s", payload)
                repository = GitRepository.from_github_full_name(full_name)
                if repository not in self._context.repositories_to_follow:
                    log.warning(
                        "Got issues event for the unfollowed repository %s", repository
                    )
                _scrape_issues(
                    self._sink,
                    repository,
                    self._context.personal_access_token,
                    issues_urls=[url],
                )
                log.info("Issues updated")
            case event:
                log.warning("Received unknown event=%s ", event)

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


def _validate_signature(data: bytes, signature: str, secret: str) -> bool:
    expected_signature = hmac.new(
        secret.encode("utf-8"), msg=data, digestmod=hashlib.sha256
    ).hexdigest()

    return hmac.compare_digest(f"sha256={expected_signature}", signature)
