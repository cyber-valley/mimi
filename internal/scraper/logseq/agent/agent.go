package agent

import (
	"context"
	"log"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"

	"mimi/internal/scraper/logseq/rag"
)

type LogseqAgent struct {
	g   *genkit.Genkit
	rag rag.RAG
}

func New(ctx context.Context, rag rag.RAG) LogseqAgent {
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

	// Done
	return LogseqAgent{
		g:   g,
		rag: rag,
	}
}

func (a LogseqAgent) Answer(query string) (string, error) {
	panic("not implemented")
}
