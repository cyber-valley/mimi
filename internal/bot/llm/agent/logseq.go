package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"mimi/internal/scraper/logseq/db"
)

type LogseqAgent struct {
	g              *genkit.Genkit
	q              *db.Queries
	retrievePrompt *ai.Prompt
	evalPrompt     *ai.Prompt
}

func NewLogseqAgent(g *genkit.Genkit, q *db.Queries) LogseqAgent {
	// Fail fast if prompt wasn't found
	retrieve := genkit.LookupPrompt(g, "logseq-retrieve")
	if retrieve == nil {
		log.Fatal("no prompt named 'logseq-retrieve' found")
	}
	eval := genkit.LookupPrompt(g, "logseq-eval")
	if eval == nil {
		log.Fatal("no prompt named 'logseq-eval' found")
	}

	return LogseqAgent{
		g:              g,
		q:              q,
		retrievePrompt: retrieve,
		evalPrompt:     eval,
	}
}

func (a LogseqAgent) GetInfo() Info {
	return Info{
		Name: "logseq",
		Description: `Knows all about cyber valley's history.
		Capable of answering to questions about flora and fauna, main goals and mindsets`,
	}
}

func (a LogseqAgent) Run(ctx context.Context, query string, msgs ...*ai.Message) (*ai.ModelResponse, error) {
	// Find relative pages
	titles, err := a.q.FindTitles()
	if err != nil {
		return nil, fmt.Errorf("failed to answer to query with %w", err)
	}
	titleDocs := make([]*ai.Document, len(titles))
	for i, t := range titles {
		titleDocs[i] = ai.DocumentFromText(t, map[string]any{})
	}

	// Ask LLM to filter only relevant pages
	resp, err := a.retrievePrompt.Execute(
		ctx,
		ai.WithDocs(titleDocs...),
		ai.WithMessages(msgs...),
		ai.WithInput(map[string]any{"query": query}),
	)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed with %w", err)
	}
	var relevantPages map[string][]string
	if err := resp.Output(&relevantPages); err != nil {
		return nil, fmt.Errorf("failed to parse LLM output with %w", err)
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
	outer:
		for _, rel := range rels {
			for _, doc := range docs {
				if doc.Metadata["title"].(string) == rel.Title {
					continue outer
				}
			}
			docs = append(docs, ai.DocumentFromText(rel.Content, map[string]any{"title": rel.Title}))
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to fetch relevant pages with %w", errors.Join(errs...))
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("there is no any relevant page")
	}
	slog.Info("relevant documents", "length", len(docs))

	// Evaluate final prompt
	resp, err = a.evalPrompt.Execute(
		ctx,
		ai.WithDocs(docs...),
		ai.WithInput(map[string]any{"query": query}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate final step with %w", err)
	}

	return resp, nil
}
