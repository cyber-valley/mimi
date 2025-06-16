package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"mimi/internal/scraper/logseq"
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

	// Synchronize LogSeq contents
	if err := g.Sync(ctx); err != nil {
		log.Fatalf("failed to sync graph with %s", err)
	}
}
