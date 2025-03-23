import os

import httpx
from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import AIMessage, HumanMessage
from langchain_core.outputs import ChatGeneration, ChatResult


class OpenRouterAutoLLM(BaseChatModel):
    """Optimized LLM class for OpenRouter with Turso vector search (RAG)."""

    def __init__(self):
        super().__init__()
        self.model = os.environ["OPENROUTER_MODEL"]
        self.api_key = os.environ["OPENROUTER_API_KEY"]
        self.api_url = os.environ["OPENROUTER_API_URL"]

    async def _generate(
        self, messages: list[HumanMessage], stop=None, **kwargs
    ) -> ChatResult:
        """Handles async LLM request with RAG context retrieval."""
        return await self._call(messages, stop=stop, **kwargs)

    async def _call(self, messages, stop=None, **kwargs) -> ChatResult:
        """Retrieves relevant context and queries OpenRouter."""
        query = messages[-1].content
        context = await self._retrieve_relevant_context(query)

        openrouter_messages = [
            {"role": "system", "content": f"Relevant context:\n{context}"}
        ] + [{"role": msg.role, "content": msg.content} for msg in messages]

        payload = {
            "model": self.model,
            "messages": openrouter_messages,
            "temperature": kwargs.get("temperature", self._adaptive_temperature(query)),
            "max_tokens": kwargs.get("max_tokens", 1024),
        }

        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }

        async with httpx.AsyncClient() as client:
            response = await client.post(
                self.api_url, json=payload, headers=headers, timeout=30
            )
            response.raise_for_status()
            response_data = response.json()

        ai_response = response_data["choices"][0]["message"]["content"]
        return ChatResult(
            generations=[ChatGeneration(message=AIMessage(content=ai_response))]
        )

    async def _retrieve_relevant_context(self, query: str) -> str:
        """Fetches the most relevant embeddings from Turso."""
        return ""

    def _adaptive_temperature(self, query: str) -> float:
        """Adjusts temperature dynamically based on query length."""
        length = len(query.split())
        return 0.7 if length < 20 else 0.5 if length < 50 else 0.3

    def _llm_type(self):
        """Returns the model type for logging/debugging."""
        return self.model
