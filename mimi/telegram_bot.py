import os

import telebot

from mimi.rag_chat_prompt import complete


def create_bot(graph):
    bot = telebot.TeleBot(os.environ["TELEGRAM_BOT_API_TOKEN"])

    @bot.message_handler(commands=["help", "start"])
    def send_welcome(message):
        bot.reply_to(
            message, "Hi there! Ask me anything, and I'll do my best to answer."
        )

    @bot.message_handler(func=lambda message: True)
    def handle_message(message):
        query = message.text
        answer = complete(graph, query)
        bot.reply_to(message, answer)

    return bot
