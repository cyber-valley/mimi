package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"mimi/internal/scraper/logseq"
	"mimi/internal/scraper/logseq/db"
	"os"
	"os/signal"
)

func main() {
	// Glog initialization
	flag.Lookup("stderrthreshold").Value.Set("INFO")
	flag.Parse()

	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Connect to CozoDB
	q := db.New()
	err := q.CreateRelations()
	if err != nil {
		log.Fatalf("failed to create relations with %s", err)
	}

	// LogSeq graph initialization
	g, err := logseq.New(ctx, q, "/home/user/code/clone/cvland")
	if err != nil {
		log.Fatalf("failed to create graph with %s", err)
	}

	// Synchronize LogSeq contents
	if err := g.Sync(); err != nil {
		log.Fatalf("failed to sync graph with %s", err)
	}

	// Process user query with LLM
	contents, err := g.Retrieve("genesis")
	if err != nil {
		log.Fatalf("failed to retrieve pages with %s", err)
	}
	var totalSize int
	for _, content := range contents {
		totalSize += len(content)
		slog.Info("retrieved page", "content", content)
	}
	slog.Info("total data retrieved", "amount", totalSize)
}
