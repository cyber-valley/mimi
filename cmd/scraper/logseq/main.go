package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"mimi/internal/provider/logseq"
	"mimi/internal/provider/logseq/db"
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
	g := logseq.NewRegexGraph("/home/user/code/clone/cvland")

	// Synchronize LogSeq contents
	if err := logseq.Sync(ctx, g, q); err != nil {
		log.Fatalf("failed to sync graph with %s", err)
	}
}
