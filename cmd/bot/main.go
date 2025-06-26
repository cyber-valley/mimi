package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"mimi/internal/bot"
	"mimi/internal/config"
)

const (
	logseqGraphEnv = "LOGSEQ_GRAPH_PATH"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	config := config.FromEnv()
	logseqPath := os.Getenv(logseqGraphEnv)
	if logseqPath != "" {
		log.Fatalf("env variable %s is missing", logseqGraphEnv)
	}
	bot.Start(ctx, config.TelegramBotApiToken, logseqPath)
}
