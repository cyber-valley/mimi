package main

import (
	"context"
	"flag"
	"mimi/internal/scraper/logseq"
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
