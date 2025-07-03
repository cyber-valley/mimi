package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/bot"
	ghscraper "mimi/internal/provider/github/scraper"
	tgscraper "mimi/internal/provider/telegram/scraper"
)

const (
	logseqGraphEnv      = "LOGSEQ_GRAPH_PATH"
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

	g, err := genkit.Init(ctx,
		genkit.WithPlugins(&googlegenai.GoogleAI{}),
		genkit.WithDefaultModel("googleai/gemini-2.0-flash"),
	)
	if err != nil {
		log.Fatalf("could not initialize Genkit: %s", err)
	}

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to postgres with: %s", err)
	}

	// Run scrapers
	go tgscraper.Run(ctx, pool, g)
	go ghscraper.Run(ctx, 8000, pool)

	// Run Telegram bot
	bot.Start(ctx, tgBotToken, logseqPath)
}
