package rag

import (
	"context"
	"log"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"

	"mimi/internal/scraper/logseq/db"
)

type RAG struct {
	g *genkit.Genkit
	q *db.Queries
	e ai.Embedder
}

func New(ctx context.Context, q *db.Queries) RAG {
	// Init genkit
	g, err := genkit.Init(ctx)
	if err != nil {
		log.Fatalf("could not initialize Genkit: %v", err)
	}

	// Setup OpenAI embedder
	oaiKey := os.Getenv("OPENAI_API_KEY")
	if oaiKey == "" {
		log.Fatal("OPENAI_API_TOKEN env var should be set")
	}
	oai := openai.OpenAI{
		APIKey: oaiKey,
	}
	if err = oai.Init(ctx, g); err != nil {
		log.Fatalf("failed to init OpenAI plugin with %s", err)
	}
	embedder := oai.Embedder(g, "text-embedding-3-small")

	// Done
	return RAG{
		g: g,
		q: q,
		e: embedder,
	}
}

func (r RAG) Index(query string) error {
	panic("not implemented")
}

func (r RAG) Retrieve(query string) (*ai.RetrieverResponse, error) {
	panic("not implemented")
}
