package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/cozodb/cozo-lib-go"
	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/provider/github/scraper"
	"mimi/internal/provider/logseq"
	"mimi/internal/provider/logseq/db"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to postgres with: %s", err)
	}

	var hooks []scraper.PushEventHook

	// Setup LogSeq push event hook
	conn, err := cozo.New("mem", "", nil)
	if err != nil {
		log.Fatalf("failed to connect to cozo with %s", err)
	}
	q := db.New(conn)
	err = q.CreateRelations()
	if err != nil {
		log.Fatalf("failed to create relations with %s", err)
	}
	hooks = append(hooks, scraper.PushEventHook{
		RepoOwner: "cyber-valley",
		RepoName:  "cvland",
		Hook:      logseq.NewSyncer(q),
	})

	if err := scraper.Run(ctx, pool, hooks...); err != nil {
		log.Fatalf("failed to run GitHub scraper with %s", err)
	}
}
