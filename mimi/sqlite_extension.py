import inspect
import logging
import sqlite3
from collections.abc import Iterator
from contextlib import contextmanager
from dataclasses import dataclass, field
from typing import cast, override

log = logging.getLogger(__name__)


def create_connection_proxy[T: sqlite3.Connection](
    connection: sqlite3.Connection, proxy_cls: type[T]
) -> T:
    return cast(T, _ConnectionProxy(connection, proxy_cls))


@dataclass
class _ConnectionProxy[T: sqlite3.Connection]:
    obj: sqlite3.Connection
    proxy_cls: type[T]
    overridden_methods: dict[str, object] = field(init=False)

    def __post_init__(self) -> None:
        self.overridden_methods = {
            name: member
            for name, member in inspect.getmembers(self.proxy_cls)
            if hasattr(member, "__override__")
        }

    def __getattr__(self, name: str) -> object:
        return self.overridden_methods.get(name, getattr(self.obj, name))


class TranscationlessConnectionProxy(sqlite3.Connection):
    @override
    def commit() -> None:  # type: ignore[misc]
        pass


@contextmanager
def sqlite3_transaction(connection: sqlite3.Connection) -> Iterator[None]:
    try:
        yield
        connection.commit()
    except Exception:
        log.exception("Transaction failed")
        connection.rollback()
