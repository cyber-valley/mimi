from typing import Protocol


class DataSink[T](Protocol):
    def put(self, obj: T) -> None: ...
