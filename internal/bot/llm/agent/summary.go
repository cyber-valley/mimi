package agent

import (
	"context"
	"log"
	"log/slog"
	"fmt"
	"encoding/json"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/persist"
	"mimi/internal/scraper/github/db"
)

const (
	evalSummaryPrompt = "summary"
)

type SummaryAgent struct {
	evalPrompt     *ai.Prompt
	ghClient *db.Client
	ghOrg string
	pgPool *pgxpool.Pool
}

func NewSummaryAgent(g *genkit.Genkit, pgPool *pgxpool.Pool, ghOrg string, logseqRepoPath string) SummaryAgent {
	// Fail fast if prompt wasn't found
	eval := genkit.LookupPrompt(g, evalSummaryPrompt)
	if eval == nil {
		log.Fatalf("no prompt named '%s' found", evalSummaryPrompt)
	}

	return SummaryAgent{
		pgPool: pgPool,
		ghClient: db.New("https://api.github.com/graphql"),
		ghOrg: ghOrg,
		evalPrompt:     eval,
	}
}

func (a SummaryAgent) GetInfo() Info {
	return Info{
		Name: "summary",
		Description: `Provides overall summary across all available resources`,
	}
}

// TODO: Most of the retrieving could be executed in parallel
func (a SummaryAgent) Run(ctx context.Context, query string, msgs ...*ai.Message) (*ai.ModelResponse, error) {
	// TODO: Extract period from the query
	// TODO: Filter Telegram messages by period
	// TODO: Filter GitHub messages by period
	var docs []*ai.Document

	// Retrieve GitHub projects statuses
	issues := make(map[string][]db.Issue)
	// TODO: Probably should be moved into `GetOrgProject`
	columnNames := []string{"monthly plan", "ordered", "shipped"}
	// TODO: Should it be persisted in DB?
	projects := map[string]int{
		"rockets": 2,
		"supply": 3,
		"inventory": 24,
		"devops force": 33,
	}
	// Fetch issues for each project
	for title, projID := range projects {
		tmp, err := a.ghClient.GetOrgProject(ctx, a.ghOrg, projID, columnNames)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch supply board state with %w", err)
		}
		slog.Info("fetched GitHub issues", "project", title, "lenght", len(tmp))
		issues[title] = tmp
	}
	blob, err := json.Marshal(issues)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GitHub projects info with %w", err)
	}
	docs = append(docs, ai.DocumentFromText(string(blob), map[string]any{"info": "GitHub projects issues"}))

	// Retrieve Telegram info
	q := persist.New(a.pgPool)
	messages, err := q.FindTelegramMessages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Telegram message from DB with %w", err)
	}
	slog.Info("retrieved Telegram messages", "length", len(messages))
	blob, err = json.Marshal(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Telegram messages with %w", err)
	}
	docs = append(docs, ai.DocumentFromText(string(blob), map[string]any{"info": "Related telegram messages"}))

	return a.evalPrompt.Execute(ctx, ai.WithDocs(docs...))
}
