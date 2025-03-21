from typing import Protocol


class DataSink[T](Protocol):
    async def puth(self, obj: T) -> None: ...
