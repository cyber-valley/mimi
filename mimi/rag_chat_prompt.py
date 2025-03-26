import functools
import logging
from dataclasses import dataclass
from typing import Any, Final, Literal

from langchain_core.documents import Document
from langchain_core.language_models import BaseChatModel
from langchain_core.prompts import PromptTemplate
from langchain_core.vectorstores import VectorStore
from langgraph.graph import START, StateGraph
from langgraph.graph.state import CompiledStateGraph
from result import Err, Ok, Result

log = logging.getLogger(__name__)

_TEMPLATE: Final = """
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
    context: list[str]
    answer: str


def setup_graph(vector_store: VectorStore, llm: BaseChatModel) -> CompiledStateGraph:
    graph_builder = StateGraph(State)
    graph_builder.add_node(
        _retrieve.__name__, functools.partial(_retrieve, vector_store=vector_store)
    )
    graph_builder.add_node(
        _generate.__name__, functools.partial(_generate, llm=llm, template=_TEMPLATE)
    )
    graph_builder.add_edge(START, _retrieve.__name__)
    return graph_builder.compile()


class LangGraphInvokationError(Exception):
    def __init__(self, error: Exception) -> None:
        super().__init__(f"Failed to invoke langgraph with {error}")


class UnsupportedLangGraphFormatError(Exception):
    def __init__(self, info: Any) -> None:
        super().__init__(f"Got unsupported answer format: {info}")


RagCompletionError = LangGraphInvokationError | UnsupportedLangGraphFormatError


def complete(graph: CompiledStateGraph, query: str) -> Result[str, RagCompletionError]:
    # Langgraph doesn't provide base exception and
    # inherits all exceptions for the `Exception` class
    # Here we can get one of those or any possible from the
    # langgraph's dependencies
    try:
        result = graph.invoke({"question": query})
    except Exception as e:
        return Err(LangGraphInvokationError(e))

    match result:
        case {"result": answer} if isinstance(answer, str):
            return Ok(answer)
        case unknown:
            return Err(UnsupportedLangGraphFormatError(unknown))


def _retrieve(
    state: State, vector_store: VectorStore
) -> dict[Literal["context"], list[Document]]:
    retrieved_docs = vector_store.similarity_search(state.question)
    return {"context": retrieved_docs}


def _generate(
    state: State, llm: BaseChatModel, template: str
) -> dict[Literal["answer"], str | list[Any]]:
    prompt = PromptTemplate.from_template(template)
    messages = prompt.invoke({"question": state.question, "context": state.context})
    response = llm.invoke(messages)
    return {"answer": response.content}
