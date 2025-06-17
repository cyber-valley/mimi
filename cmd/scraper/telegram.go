package main

import (
	"context"
	"flag"
	"os"
	"os/signal"

	"github.com/golang/glog"
	"github.com/jackc/pgx/v5"
	"mimi/internal/scraper/telegram"
)

func main() {
	flag.Lookup("stderrthreshold").Value.Set("INFO")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		glog.Fatal("failed to connect to postgres with: ", err)
	}
	defer conn.Close(ctx)

	if err := telegram.Run(ctx, conn); err != nil {
		glog.Fatalf("error: %+v", err)
	}

	glog.Warning("tg scraper exited without any error")
}
