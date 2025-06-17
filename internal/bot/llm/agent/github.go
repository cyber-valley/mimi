package agent

import (
	"context"
	"fmt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"mimi/internal/scraper/github/db"
)

type GitHubAgent struct {
	g      *genkit.Genkit
	c      *db.Client
	prompt *ai.Prompt
}

func NewGitHubAgent(g *genkit.Genkit) GitHubAgent {
	prompt := genkit.LookupPrompt(g, "supply-state")
	if prompt == nil {
		panic("failed to load 'supply-state' prompt")
	}
	return GitHubAgent{
		g:      g,
		c:      db.New("https://api.github.com/graphql"),
		prompt: prompt,
	}
}

func (a GitHubAgent) GetInfo() Info {
	return Info{
		Name:        "github",
		Description: `Capabled of answering about supply tasks state`,
	}
}

func (a GitHubAgent) Run(ctx context.Context, query string, msgs ...*ai.Message) (*ai.ModelResponse, error) {
	// Fetch GitHub board state
	columnNames := []string{"monthly plan", "ordered", "shipped"}
	issues, err := a.c.GetOrgProject(ctx, "cyber-valley", 3, columnNames)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch supply board state with %w", err)
	}

	// Setup context
	docs := make([]*ai.Document, len(issues))
	for i, issue := range issues {
		docs[i] = ai.DocumentFromText(issue.Body, map[string]any{
			"title": issue.Title,
			"url":   issue.URL,
			"state": issue.State,
		})
	}

	// Eval prompt
	resp, err := a.prompt.Execute(
		ctx,
		ai.WithDocs(docs...),
		ai.WithMessages(msgs...),
		ai.WithInput(map[string]any{"query": query}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate final step with %w", err)
	}
	return resp, nil
}
