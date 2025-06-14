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
	flag.Lookup("stderrthreshold").Value.Set("INFO")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	q := db.New()
	err := q.CreateRelations()
	if err != nil {
		log.Fatalf("failed to create relations with %s", err)
	}

	g, err := logseq.New(ctx, q, "/home/user/code/clone/cvland")
	if err != nil {
		log.Fatalf("failed to create graph with %s", err)
	}

	if err := g.Sync(); err != nil {
		log.Fatalf("failed to sync graph with %s", err)
	}

	contents, err := g.Retrieve("genesis")
	if err != nil {
		log.Fatalf("failed to retrieve pages with %s", err)
	}
	var totalSize int
	for _, content := range contents {
		totalSize += len(content)
		slog.Info("retrieved page", "content", content)
	}
	slog.Info("total data retireved", "amount", totalSize)
}
