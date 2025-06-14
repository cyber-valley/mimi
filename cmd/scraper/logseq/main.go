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

	q := db.New()
	err := q.CreateRelations()
	if err != nil {
		log.Fatalf("failed to create relations with %s", err)
	}
	err = q.SavePage(db.Page{
		Title:   "Test page",
		Content: "Some page text",
		Props: map[string]string{
			"tags":    "lol",
			"aliases": "kek",
		},
		Refs: []string{"Another page"},
	})
	if err != nil {
		log.Fatalf("failed to save page with %s", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	logseq.SyncGraph(ctx, "/home/user/code/clone/cvland")
}
