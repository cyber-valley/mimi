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

	logseq.SyncGraph(ctx, "/home/user/code/clone/cvland")
}
