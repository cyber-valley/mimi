import os

from functools import lru_cache
from langchain_openai import ChatOpenAI

@lru_cache(maxsize=1)
def get_openrouter_chat_model():
    return ChatOpenAI(
        openai_api_key=os.getenv("OPENROUTER_API_KEY"),
        openai_api_base=os.getenv("OPENROUTER_BASE_URL"),
        model_name=os.getenv("OPENROUTER_MODEL"),
    )