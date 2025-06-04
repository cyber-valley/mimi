package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/golang/glog"
	"mimi/internal/scraper/tgscraper"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := tgscraper.Run(ctx); err != nil {
		glog.Fatalf("error: %+v", err)
	}

	glog.Warning("tg scraper exited without any error")
}
