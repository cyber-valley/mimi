package agent

import (
	"context"
	"fmt"
	"log"
	"encoding/json"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"mimi/internal/scraper/github/db"
)

const (
	githubEvalPromptName = "github-board-eval"
	filterProjectsPromptName = "github-board-filter"
)

type GitHubAgent struct {
	g      *genkit.Genkit
	c      *db.Client
	org string
	eval *ai.Prompt
	projectsFilter *ai.Prompt
}

func NewGitHubAgent(g *genkit.Genkit, org string) GitHubAgent {
	// Fail fast if any dot prompt doesn't exist
	eval := genkit.LookupPrompt(g, githubEvalPromptName)
	if eval == nil {
		log.Fatalf("failed to load '%s' prompt", githubEvalPromptName)
	}
	projectsFilter := genkit.LookupPrompt(g, filterProjectsPromptName)
	if projectsFilter == nil {
		log.Fatalf("failed to load '%s' prompt", filterProjectsPromptName)
	}

	c := db.New("https://api.github.com/graphql")

	return GitHubAgent{
		g:      g,
		c:      c,
		org: org,
		eval: eval,
		projectsFilter: projectsFilter,
	}
}

func (a GitHubAgent) GetInfo() Info {
	return Info{
		Name:        "github",
		Description: `Capabled of answering about supply tasks state`,
	}
}

func (a GitHubAgent) Run(ctx context.Context, query string, msgs ...*ai.Message) (*ai.ModelResponse, error) {
	// Gather existing projects
	projects, err := a.c.ListProjects(context.Background(), a.org)
	if err != nil {
		return nil, fmt.Errorf("failed to get projects list for '%s' with %w", a.org, err)
	}
	projectsBlob, err := json.Marshal(projects)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize GitHub projects list with %w", err)
	}

	// Find out target projects
	resp, err := a.projectsFilter.Execute(
		ctx,
		ai.WithDocs(ai.DocumentFromText(string(projectsBlob), map[string]any{})),
		ai.WithMessages(msgs...),
		ai.WithInput(map[string]any{"query": query}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to filter related GitHub projects with %w", err)
	}
	var targetProjects []projectInfo
	if err := resp.Output(&targetProjects); err != nil {
		return nil, fmt.Errorf("failed to unmarshal filtered projects '%s' with %w", resp.Text(), err)
	}

	// Fetch GitHub board state
	columnNames := []string{"monthly plan", "ordered", "shipped"}
	issues := make(map[string][]db.Issue)
	for _, info := range targetProjects {
		tmp, err := a.c.GetOrgProject(ctx, a.org, info.Id, columnNames)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch supply board state with %w", err)
		}
		issues[info.Title] = tmp
	}

	// Setup context
	var docs []*ai.Document
	for title, issues := range issues {
		for _, issue := range issues {
			docs = append(docs, ai.DocumentFromText(issue.Body, map[string]any{
				"title": issue.Title,
				"url":   issue.URL,
				"state": issue.State,
				"projectTitle": title,
			}))
		}
	}

	// Eval prompt
	resp, err = a.eval.Execute(
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

type projectInfo struct {
	Id int `json:"id"`
	Title string `json:"title"`
}
