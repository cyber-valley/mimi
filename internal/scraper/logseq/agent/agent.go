package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"

	"mimi/internal/scraper/logseq/rag"
)

type LogseqAgent struct {
	g   *genkit.Genkit
	rag rag.RAG
	p   *ai.Prompt
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
	p := genkit.LookupPrompt(g, "logseq-retrieve")
	if p == nil {
		log.Fatal("no prompt named 'logseq-retrieve' found")
	}

	// Done
	return LogseqAgent{
		g:   g,
		rag: rag,
		p:   p,
	}
}

func (a LogseqAgent) Answer(ctx context.Context, query string) (string, error) {
	docs, err := a.rag.Retrieve(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve context with %w", err)
	}
	answer, err := a.p.Execute(
		ctx,
		ai.WithDocs(docs...),
		ai.WithInput(map[string]any{"query": query}),
	)
	if err != nil {
		return "", fmt.Errorf("LLM request failed with %w", err)
	}
	return answer.Text(), nil
}
