package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/cozodb/cozo-lib-go"

	"mimi/internal/provider/logseq"
	"mimi/internal/provider/logseq/db"
)

func main() {
	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Connect to CozoDB
	conn, err := cozo.New("mem", "", nil)
	if err != nil {
		log.Fatalf("failed to connect to cozo with %s", err)
	}
	q := db.New(conn)
	err = q.CreateRelations()
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
