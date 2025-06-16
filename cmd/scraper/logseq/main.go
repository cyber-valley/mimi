package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"

	"mimi/internal/scraper/logseq"
	"mimi/internal/scraper/logseq/agent"
	"mimi/internal/scraper/logseq/db"
	"mimi/internal/scraper/logseq/rag"
)

func main() {
	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Connect to CozoDB
	q := db.New()
	err := q.CreateRelations()
	if err != nil {
		log.Fatalf("failed to create relations with %s", err)
	}

	// Initialize
	rag := rag.New(ctx, q)
	g, err := logseq.New(ctx, q, rag, "/home/user/code/clone/cvland")
	if err != nil {
		log.Fatalf("failed to create graph with %s", err)
	}
	agent := agent.New(ctx, q, g.Retrieve)

	// Synchronize LogSeq contents
	if err := g.Sync(ctx); err != nil {
		log.Fatalf("failed to sync graph with %s", err)
	}

	slog.Info("Quering LLM")
	response, err := agent.Answer(ctx, "What is cyber valley")
	if err != nil {
		log.Fatalf("failed to query LLM with %s", err)
	}
	slog.Info("LLM", "response", response)
}
