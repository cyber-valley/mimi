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
)

type LogseqAgent struct {
	g        *genkit.Genkit
	q        *db.Queries
	retrieve *ai.Prompt
	eval     *ai.Prompt
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
	titleDocs := make([]*ai.Document, len(titles))
	for i, t := range titles {
		titleDocs[i] = ai.DocumentFromText(t, map[string]any{})
	}

	// Ask LLM to filter only relevant pages
	resp, err := a.retrieve.Execute(
		ctx,
		ai.WithDocs(titleDocs...),
		ai.WithInput(map[string]any{"query": query}),
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
	var docs []*ai.Document
	for _, t := range relevantPages["titles"] {
		rels, err := a.q.FindRelatives(t, 5)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, rel := range rels {
			docs = append(docs, ai.DocumentFromText(rel.Content, map[string]any{"title": rel.Title}))
		}
	}
	if len(errs) > 0 {
		return "", fmt.Errorf("failed to fetch relevant pages with %w", errors.Join(errs...))
	}
	if len(docs) == 0 {
		return "", fmt.Errorf("there is no any relevant page")
	}
	slog.Info("relevant documents", "length", len(docs))

	// Evaluate final prompt
	resp, err = a.eval.Execute(
		ctx,
		ai.WithDocs(docs...),
		ai.WithInput(map[string]any{"query": query}),
	)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate final step with %w", err)
	}

	return resp.Text(), nil
}
