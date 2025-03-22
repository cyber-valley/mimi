import functools
import json
import logging
import time
from collections.abc import Iterable
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path
from string import Template
from typing import Any, NoReturn

import requests
import tenacity
from rss_parser import RSSParser

from mimi import DataOrigin, DataSink

from . import DataScraperMessage

log = logging.getLogger(__name__)
XMessage = functools.partial(DataScraperMessage, origin=DataOrigin.X)
goole_news_rss_url_template = Template(
    "https://news.google.com/rss/search?q=site:x.com/$account"
)


@dataclass
class XScraperContext:
    user_tweets_json_directory: Path
    accounts_to_follow: list[str]
    poll_interval: timedelta


def scrape(context: XScraperContext, sink: DataSink[DataScraperMessage]) -> NoReturn:
    # Process static files
    for path in context.user_tweets_json_directory.glob("**/*.json"):
        for message in _parse_user_tweets(json.loads(path.read_text())):
            sink.put(message)

    # Start polling RSS feeds
    while True:
        for account in context.accounts_to_follow:
            log.info("Starting parsing of %s", account)
            feed_url = goole_news_rss_url_template.substitute(account=account)
            for message in _parse_rss_feed(feed_url):
                sink.put(message)
            log.info("Finished parsing of %s", account)

        time.sleep(context.poll_interval.total_seconds())


def _parse_user_tweets(data: dict[str, Any]) -> Iterable[DataScraperMessage]:
    for value in data.values():
        if isinstance(value, dict):
            yield from _parse_user_tweets(value)

        elif isinstance(value, list):
            for item in value:
                if not isinstance(item, dict):
                    continue
                yield from _parse_user_tweets(item)

    if text := data.get("full_text"):
        assert isinstance(text, str)
        created_at = data.get("created_at")
        if created_at is None:
            log.error("Failed to find created date in user tweet")
            return

        try:
            pub_date = datetime.strptime(created_at, "%a %b %d %H:%M:%S %z %Y")
        except ValueError:
            log.exception("Failed to parse publication date")
            return

        yield XMessage(data=text, scraped_at=datetime.now(UTC), pub_date=pub_date)


@tenacity.retry(
    retry=tenacity.retry_if_exception_type(requests.HTTPError)
    | tenacity.retry_if_exception_type(requests.Timeout),
    wait=tenacity.wait_random_exponential(multiplier=1, max=60),
)
def _parse_rss_feed(url: str) -> Iterable[DataScraperMessage]:
    response = requests.get(url, timeout=10)
    response.raise_for_status()
    parser = RSSParser.parse(response.text)
    for item in parser.channel.items:
        try:
            pub_date = datetime.strptime(
                item.pub_date, "%a, %d %b %Y %H:%M:%S %Z"
            ).replace(tzinfo=UTC)
        except ValueError:
            log.exception("Failed to parse publication date %s", item.pub_date)
            continue
        yield XMessage(data=item.title, scraped_at=datetime.now(UTC), pub_date=pub_date)
