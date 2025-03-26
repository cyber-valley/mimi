import logging
import os

import telebot
from langgraph.graph.state import CompiledStateGraph
from telebot.types import Message

from mimi.rag_chat_prompt import complete

log = logging.getLogger(__name__)


def create_bot(graph: CompiledStateGraph) -> None:
    bot = telebot.TeleBot(os.environ["TELEGRAM_BOT_API_TOKEN"])

    @bot.message_handler(commands=["start"])
    def send_welcome(message: Message) -> None:
        bot.reply_to(
            message, "Hi there! Ask me anything, and I'll do my best to answer."
        )

    @bot.message_handler
    def handle_message(message: Message) -> None:
        log.info(
            "[%s] Got new message from %s: %s",
            message.message_id,
            message.from_user.id,
            message.text,
        )
        try:
            answer = complete(graph, message.textCompiledStateGraph)
        # TODO: Concretize exception
        except Exception:
            log.exception("[%s] Failed to process query", message.message_id)
            return

        bot.reply_to(message, answer)
