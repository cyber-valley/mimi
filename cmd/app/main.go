package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/bot"
	ghscraper "mimi/internal/provider/github/scraper"
	tgscraper "mimi/internal/provider/telegram/scraper"
)

const (
	logseqGraphEnv      = "LOGSEQ_GRAPH_PATH"
	telegramBotTokenEnv = "TELEGRAM_BOT_API_TOKEN"
	openrouterApiKeyEnv = "OPENROUTER_API_KEY"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var missingEnvVars []string
	tgBotToken := os.Getenv(telegramBotTokenEnv)
	if tgBotToken == "" {
		missingEnvVars = append(missingEnvVars, telegramBotTokenEnv)
	}

	logseqPath := os.Getenv(logseqGraphEnv)
	if logseqPath == "" {
		missingEnvVars = append(missingEnvVars, logseqGraphEnv)
	}

	openrouterApiKey := os.Getenv(openrouterApiKeyEnv)
	if openrouterApiKey == "" {
		missingEnvVars = append(missingEnvVars, openrouterApiKeyEnv)
	}

	if len(missingEnvVars) > 0 {
		log.Fatalf("env variables %#v are missing", missingEnvVars)
	}

	g, err := genkit.Init(ctx,
		genkit.WithPlugins(
			&googlegenai.GoogleAI{},
			&openai.OpenAI{APIKey: openrouterApiKey},
		),
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
