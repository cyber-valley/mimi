package agent

import (
	"context"
	"log"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"

	"mimi/internal/scraper/logseq/db"
)

type LogseqAgent struct {
	g *genkit.Genkit
	q *db.Queries
	e ai.Embedder
}

func New(ctx context.Context, q *db.Queries) LogseqAgent {
	// Init genkit
	g, err := genkit.Init(
		ctx,
		genkit.WithPlugins(&googlegenai.GoogleAI{}),
		genkit.WithDefaultModel("googleai/gemini-2.0-flash"),
	)
	if err != nil {
		log.Fatalf("could not initialize Genkit: %v", err)
	}

	// Fail fast if prompt wasn't found
	p := genkit.LookupPrompt(g, "logseq-cozo")
	if p == nil {
		log.Fatal("no prompt named 'menu' found")
	}

	// Setup OpenAI embedder
	oaiKey := os.Getenv("OPENAI_API_TOKEN")
	if oaiKey == "" {
		log.Fatal("OPENAI_API_TOKEN env var should be set")
	}
	oai := openai.OpenAI{
		APIKey: oaiKey,
	}
	if err = oai.Init(ctx, g); err != nil {
		log.Fatalf("failed to init OpenAI plugin with %s", err)
	}
	embedder, err := oai.DefineEmbedder(g, "text-embedding-3-small")
	if err != nil {
		log.Fatalf("failed to define OpenAI embedder with %s", err)
	}

	// Done
	return LogseqAgent{
		g: g,
		q: q,
		e: embedder,
	}
}

func (a LogseqAgent) Answer(query string) (string, error) {
	panic("not implemented")
}
