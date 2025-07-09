package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openai/openai-go/option"

	"mimi/internal/bot"
	ghscraper "mimi/internal/provider/github/scraper"
	"mimi/internal/provider/logseq"
	"mimi/internal/provider/logseq/db"
	tgscraper "mimi/internal/provider/telegram/scraper"
)

const (
	logseqGraphEnv      = "LOGSEQ_GRAPH_PATH"
	telegramBotTokenEnv = "TELEGRAM_BOT_API_TOKEN"
	openrouterApiKeyEnv = "OPENROUTER_API_KEY"
	openrouterApiUrlEnv = "OPENROUTER_API_URL"
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

	openrouterBaseURL := os.Getenv(openrouterApiUrlEnv)
	if openrouterBaseURL == "" {
		missingEnvVars = append(missingEnvVars, openrouterApiUrlEnv)
	}

	if len(missingEnvVars) > 0 {
		log.Fatalf("env variables %#v are missing", missingEnvVars)
	}

	oai := &openai.OpenAI{
		APIKey: openrouterApiKey,
		Opts: []option.RequestOption{
			option.WithBaseURL(openrouterBaseURL),
		},
	}

	g, err := genkit.Init(ctx,
		genkit.WithPlugins(oai),
		genkit.WithDefaultModel("openai/google/gemini-2.5-flash"),
	)
	oai.DefineModel(g, "google/gemini-2.5-flash", ai.ModelInfo{
		Label:    "Gemini 2.5 Flash Preview 04-17",
		Versions: []string{"google/gemini-2.5-flash"},
		Supports: &ai.ModelSupports{
			Multiturn:   true,
			Tools:       true,
			ToolChoice:  true,
			SystemRole:  true,
			Media:       true,
			Constrained: ai.ConstrainedSupportNone,
		},
		Stage: ai.ModelStageStable,
	})

	if err != nil {
		log.Fatalf("could not initialize Genkit: %s", err)
	}

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to postgres with: %s", err)
	}

	// Setup LogSeq push event hook
	var hooks []ghscraper.PushEventHook
	q := db.New()
	err = q.CreateRelations()
	if err != nil {
		log.Fatalf("failed to create relations with %s", err)
	}
	hooks = append(hooks, ghscraper.PushEventHook{
		RepoOwner: "cyber-valley",
		RepoName:  "cvland",
		Hook:      logseq.NewSyncer(q),
	})

	go func() {
		err := tgscraper.Run(ctx, pool, g)
		if err != nil {
			log.Fatalf("Telegram scraper exited with %s", err)
		} else {
			slog.Info("Telegram scraper exited without an error")
		}
	}()
	go func() {
		err := ghscraper.Run(ctx, pool, hooks...)
		if err != nil {
			log.Fatalf("GitHub scraper exited with %s", err)
		} else {
			slog.Info("GitHub scraper exited without an error")
		}
	}()
	go func() {
		err := bot.Start(ctx, tgBotToken, logseqPath, g)
		if err != nil {
			log.Fatalf("Telegram bot exited with %s", err)
		} else {
			slog.Info("Telegram bot exited without an error")
		}
	}()

	<-ctx.Done()
}
