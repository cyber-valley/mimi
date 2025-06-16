package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"mimi/internal/scraper/logseq"
	"mimi/internal/scraper/logseq/agent"
	"mimi/internal/scraper/logseq/db"
	"mimi/internal/scraper/logseq/rag"
	"os"
	"os/signal"
)

func main() {
	// Glog initialization
	flag.Lookup("stderrthreshold").Value.Set("INFO")
	flag.Parse()

	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Connect to CozoDB
	q := db.New()
	err := q.CreateRelations()
	if err != nil {
		log.Fatalf("failed to create relations with %s", err)
	}

	// Initialize indexer & agent
	rag := rag.New(ctx, q)
	agent := agent.New(ctx, rag)

	// LogSeq graph initialization
	_, err = logseq.New(ctx, q, rag, "/home/user/code/clone/cvland")
	if err != nil {
		log.Fatalf("failed to create graph with %s", err)
	}

	// Synchronize LogSeq contents
	// if err := g.Sync(ctx); err != nil {
	// 	log.Fatalf("failed to sync graph with %s", err)
	// }

	response, err := agent.Answer(ctx, "Tell me about token model")
	if err != nil {
		log.Fatalf("failed to query LLM with %s", err)
	}
	slog.Info("LLM", "response", response)
}
