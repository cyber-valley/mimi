package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"

	"mimi/internal/scraper/logseq/db"
	"mimi/internal/scraper/logseq/rag"
)

type LogseqAgent struct {
	g        *genkit.Genkit
	rag      rag.RAG
	retrieve *ai.Prompt
	eval     *ai.Prompt
	q        *db.Queries
}

func New(ctx context.Context, rag rag.RAG, q *db.Queries) LogseqAgent {
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
	retrieve := genkit.LookupPrompt(g, "logseq-retrieve")
	if retrieve == nil {
		log.Fatal("no prompt named 'logseq-retrieve' found")
	}
	eval := genkit.LookupPrompt(g, "logseq-eval")
	if eval == nil {
		log.Fatal("no prompt named 'logseq-eval' found")
	}

	// Done
	return LogseqAgent{
		g:        g,
		rag:      rag,
		retrieve: retrieve,
		eval:     eval,
		q:        q,
	}
}

func (a LogseqAgent) Answer(ctx context.Context, query string) (string, error) {
	// Find relative pages
	titles, err := a.q.FindTitles()
	if err != nil {
		return "", fmt.Errorf("failed to answer to query with %w", err)
	}
	slog.Info("titles to choose", "titles", titles)

	// Ask LLM to filter only relevant pages
	resp, err := a.retrieve.Execute(
		ctx,
		ai.WithInput(map[string]any{"query": query, "titles": titles}),
	)
	if err != nil {
		return "", fmt.Errorf("LLM request failed with %w", err)
	}
	var relevantPages map[string][]string
	if err := resp.Output(&relevantPages); err != nil {
		return "", fmt.Errorf("failed to parse LLM output with %w", err)
	}
	slog.Info("relevant pages", "titles", relevantPages["titles"])

	// Fetch relevant docs
	var errs []error
	var docs []string
	for _, t := range relevantPages["titles"] {
		content, err := a.q.FindRelatives(t, 5)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		docs = append(docs, content...)
	}
	if len(errs) > 0 {
		return "", fmt.Errorf("failed to fetch relevant pages with %w", errors.Join(errs...))
	}
	if len(docs) == 0 {
		return "", fmt.Errorf("there is no any relevant page")
	}
	slog.Info("relevant documents", "length", len(docs), "data", docs)

	// Evaluate final prompt
	resp, err = a.eval.Execute(
		ctx,
		ai.WithInput(map[string]any{"query": query, "docs": docs}),
	)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate final step with %w", err)
	}

	return resp.Text(), nil
}
