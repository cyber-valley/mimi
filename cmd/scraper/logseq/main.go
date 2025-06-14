package main

import (
	"context"
	"flag"
	"log"
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
}
