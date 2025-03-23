from langchain_core.language_models import BaseChatModel
from langchain_core.vectorstores import VectorStore
from langgraph.graph import StateGraph


def complete(graph: StateGraph, query: str) -> str:
    raise NotImplementedError


def setup_graph(vector_store: VectorStore, llm: BaseChatModel) -> StateGraph:
    raise NotImplementedError
