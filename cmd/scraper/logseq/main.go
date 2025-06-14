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

	logseq.SyncGraph(ctx, q, "/home/user/code/clone/cvland")

	rels, err := q.FindRelatives("genesis", 2)
	if err != nil {
		log.Fatalf("failed to find relatives with %s", err)
	}
	slog.Info("found relatives", "amount", len(rels))
}
