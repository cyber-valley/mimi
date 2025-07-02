package main

import (
	"context"
	"os"
	"log"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"mimi/internal/provider/telegram"
	"mimi/internal/provider/telegram/scraper"
)

func main() {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to postgres with: %s", err)
	}
	defer conn.Close(ctx)

	slog.Info("Running setup")

	err = telegram.StartClient(ctx, func(s telegram.ClientState) error {
		return scraper.Setup(ctx, s.Client.API(), conn)
	})
	if err != nil {
		log.Fatalf("failed with %s", err)
	}
}
