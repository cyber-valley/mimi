from enum import StrEnum, auto


class DataOrigin(StrEnum):
    TELEGRAM = auto()
    X = auto()
    GITHUB = auto()


class EmbeddingProvider(StrEnum):
    OPENAI = auto()


class LLMProvider(StrEnum):
    OPENROUTER = auto()
    OPENAI = auto()
