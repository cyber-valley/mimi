package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/provider/github/scraper"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to postgres with: %s", err)
	}

	if err := scraper.Run(ctx, 8000, pool); err != nil {
		log.Fatalf("failed to run GitHub scraper with %s", err)
	}
}
