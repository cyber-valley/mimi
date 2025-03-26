import functools
import logging
from typing import Any, Final, Literal, TypedDict

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


class _State(TypedDict):
    question: str
    context: list[Document]
    answer: str


def init(vector_store: VectorStore, llm: BaseChatModel) -> CompiledStateGraph:
    graph_builder = StateGraph(_State)
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


class DocumentsNotFoundError(Exception):
    pass


RagCompletionError = LangGraphInvokationError | DocumentsNotFoundError


def complete(graph: CompiledStateGraph, query: str) -> Result[str, RagCompletionError]:
    # Langgraph doesn't provide base exception and
    # inherits all exceptions for the `Exception` class
    # Here we can get one of those or any possible from the
    # langgraph's dependencies
    try:
        result = graph.invoke({"question": query})
    except Exception as e:
        return Err(LangGraphInvokationError(e))

    answer = result.get("answer")
    if not answer:
        return Err(DocumentsNotFoundError())

    return Ok(answer)


def _retrieve(
    state: _State, vector_store: VectorStore
) -> dict[Literal["context"], list[Document]]:
    retrieved_docs = vector_store.similarity_search(state["question"])
    if retrieved_docs:
        log.info("Retrieved %s documents", len(retrieved_docs))
    else:
        log.warning("Retrieved zero documents")

    return {"context": retrieved_docs}


def _generate(
    state: _State, llm: BaseChatModel, template: str
) -> dict[Literal["answer"], str | list[Any]]:
    prompt = PromptTemplate.from_template(template)
    docs_content = "\n\n".join(doc.page_content for doc in state["context"])
    messages = prompt.invoke({"question": state["question"], "context": docs_content})
    response = llm.invoke(messages)
    return {"answer": response.content}
