import os

from langchain_core.language_models.chat_models import BaseChatModel
from langchain_openai import ChatOpenAI
from pydantic import SecretStr


def get_chat_model() -> BaseChatModel:
    api_key = _get_env_var("OPENROUTER_API_KEY")
    base_url = _get_env_var("OPENROUTER_BASE_URL")
    model = _get_env_var("OPENROUTER_MODEL")

    return ChatOpenAI(
        api_key=SecretStr(api_key),
        base_url=base_url,
        model=model,
    )


def _get_env_var(var_name: str) -> str:
    """Fetches and validates an environment variable."""
    value = os.environ.get(var_name, "").strip()
    if not value:
        error_msg = f"Missing required environment variable: {var_name}"
        raise OSError(error_msg)
    return value

if __name__ == "__main__":
    try:
        chat_model = get_chat_model()
        response = chat_model.invoke("Hello, world!")
        print(response)
    except ValueError as e:
        print(f"Error: {e}")