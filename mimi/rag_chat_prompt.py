import functools
from dataclasses import dataclass
from typing import Any, Literal

from langchain_core.documents import Document
from langchain_core.language_models import BaseChatModel
from langchain_core.prompts import PromptTemplate
from langchain_core.vectorstores import VectorStore
from langgraph.graph import START, StateGraph
from langgraph.graph.state import CompiledStateGraph

from .data_scraper import DataScraperMessage

template = """
You are Mimi The President of Cyber Valley, an assistant for question-answering tasks.
Use the following pieces of context to answer the question at the end.
If you don't know the answer,
just say that you don't know, don't try to make up an answer.
Keep the answer as concise as possible.

Context: {context}
Question: {question}

Answer:"""


@dataclass
class State:
    question: str
    context: list[DataScraperMessage]
    answer: str


def setup_graph(vector_store: VectorStore, llm: BaseChatModel) -> CompiledStateGraph:
    graph_builder = StateGraph(State)
    graph_builder.add_node(
        "retrieve", functools.partial(_retrieve, vector_store=vector_store)
    )
    graph_builder.add_node("generate", functools.partial(_generate, llm=llm))
    graph_builder.add_edge(START, "retrieve")
    return graph_builder.compile()


def complete(graph: CompiledStateGraph, query: str) -> str:
    result = graph.invoke({"question": query})
    answer = result["answer"]
    assert isinstance(answer, str), f"Got unexpected {answer=}"
    return answer


def _retrieve(
    state: State, vector_store: VectorStore
) -> dict[Literal["context"], list[Document]]:
    retrieved_docs = vector_store.similarity_search(state.question)
    return {"context": retrieved_docs}


def _generate(
    state: State, llm: BaseChatModel
) -> dict[Literal["answer"], str | list[Any]]:
    prompt = PromptTemplate.from_template(template)
    messages = prompt.invoke({"question": state.question, "context": state.context})
    response = llm.invoke(messages)
    return {"answer": response.content}
