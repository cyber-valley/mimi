package main

import (
	"context"
	"os"
	"os/signal"

	"mimi/internal/bot"
	"mimi/internal/config"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	config := config.FromEnv()
	bot.Start(ctx, config.TelegramBotApiToken)
}
