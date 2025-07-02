package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"mimi/internal/bot"
)

const (
	logseqGraphEnv = "LOGSEQ_GRAPH_PATH"
	telegramBotTokenEnv = "TELEGRAM_BOT_API_TOKEN"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	tgBotToken := os.Getenv(telegramBotTokenEnv)
	if tgBotToken == "" {
		log.Fatalf("env variable %s is missing", telegramBotTokenEnv)
	}

	logseqPath := os.Getenv(logseqGraphEnv)
	if logseqPath == "" {
		log.Fatalf("env variable %s is missing", logseqGraphEnv)
	}

	// TODO: Run scrapers
	// TODO: Run telegram setup

	// Run Telegram bot
	bot.Start(ctx, tgBotToken, logseqPath)
}
