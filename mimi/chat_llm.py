import os

from langchain_core.language_models.chat_models import BaseChatModel
from langchain_openai import ChatOpenAI


def get_chat_model() -> BaseChatModel:
    return ChatOpenAI(
        api_key=os.environ["OPENROUTER_API_KEY"],
        base_url=os.environ["OPENROUTER_BASE_URL"],
        model=os.environ["OPENROUTER_MODEL"],
    )
