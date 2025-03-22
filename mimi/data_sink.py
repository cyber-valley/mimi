from typing import Protocol


class DataSink[T](Protocol):
    async def put(self, obj: T) -> None: ...
