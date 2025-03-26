import logging
import os
from typing import assert_never

import telebot
from langgraph.graph.state import CompiledStateGraph
from result import Err, Ok
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

    @bot.message_handler()
    def handle_message(message: Message) -> None:
        log.info(
            "[%s] Got new message from %s: %s",
            message.message_id,
            message.from_user.id if message.from_user else None,
            message.text,
        )
        if not message.text:
            bot.reply_to(message, "I know how to process text messages only.")
            return

        match complete(graph, message.text):
            case Ok(answer):
                bot.reply_to(message, answer)
            case Err(err):
                log.error(
                    "[%s] Failed to process query with %s", message.message_id, err
                )
                bot.reply_to(message, "Failed to process given message")
            case unreachable:
                assert_never(unreachable)
