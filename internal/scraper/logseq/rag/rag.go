package rag

import (
	"context"
	"fmt"
	"log"
	"log/slog"
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

func (r RAG) Embed(ctx context.Context, content string) ([]float32, error) {
	slog.Info("calculating embedding for content", "size", len(content))
	v := make([]float32, 1536)
	if len(content) == 0 {
		return v, nil
	}
	resp, err := r.e.Embed(ctx, &ai.EmbedRequest{
		Input: []*ai.Document{
			&ai.Document{
				Content: []*ai.Part{
					&ai.Part{
						Kind: ai.PartText,
						Text: content,
					},
				},
			},
		},
	})
	if err != nil {
		return v, fmt.Errorf("failed to embed content with %w", err)
	}
	if len(resp.Embeddings) > 1 {
		return v, fmt.Errorf("got more embeddings than expected %d", len(resp.Embeddings))
	}
	return resp.Embeddings[0].Embedding, nil
}

func (r RAG) Retrieve(ctx context.Context, query string) (docs []*ai.Document, _ error) {
	// Embed query
	vec, err := r.Embed(ctx, query)
	if err != nil {
		return docs, fmt.Errorf("failed to embed for retrieve with %w", err)
	}

	// Query CozoDB
	pages, err := r.q.FindSimilarPages(vec)
	if err != nil {
		return docs, fmt.Errorf("failed to find similar pages with %w", err)
	}
	slog.Info("found similar pages", "len", len(pages))

	for _, page := range pages {
		slog.Info("retrieved", "title", page.Title, "content", page.Content)
		docs = append(docs, &ai.Document{
			Content: []*ai.Part{ai.NewDataPart(page.Content)},
			Metadata: map[string]any{
				"title": page.Title,
			},
		})
	}
	return docs, nil
}
