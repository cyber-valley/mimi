package agent

import (
	"strings"
	"context"
	"log"
	"log/slog"
	"fmt"
	"encoding/json"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/pgtype"

	"mimi/internal/persist"
	"mimi/internal/scraper/github/db"
)

const (
	evalSummaryPrompt = "summary"
	periodExtractorPrompt = "period-extractor"
)

type SummaryAgent struct {
	evalPrompt     *ai.Prompt
	periodExtractor     *ai.Prompt
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

	periodExtractor := genkit.LookupPrompt(g, periodExtractorPrompt)
	if periodExtractor == nil {
		log.Fatalf("no prompt named '%s' found", periodExtractorPrompt)
	}

	return SummaryAgent{
		pgPool: pgPool,
		ghClient: db.New("https://api.github.com/graphql"),
		ghOrg: ghOrg,
		evalPrompt:     eval,
		periodExtractor: periodExtractor,
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
	resp, err := a.periodExtractor.Execute(ctx, ai.WithInput(map[string]any{"query": query}))
	if err != nil {
		return nil, fmt.Errorf("failed to extract period from query '%s' with %w", query, err)
	}
	period := strings.TrimSuffix(resp.Text(), "\n")
	slog.Info("generating summary", "period", period)

	since := time.Now()
	switch period {
	default:
		return nil, fmt.Errorf("unexpected period '%s'", period)
	case "month":
		since = since.AddDate(0, 0, -30)
	case "week":
		since = since.AddDate(0, 0, -7)
	case "day":
		since = since.AddDate(0, 0, -1)
	}
	// TODO: Filter Telegram messages by period
	// TODO: Filter GitHub messages by period
	var docs []*ai.Document

	// Retrieve GitHub projects statuses
	issues := make(map[string][]db.Issue)
	// TODO: Should it be persisted in DB?
	projects := map[string]int{
		"rockets": 2,
		"supply": 3,
		"inventory": 24,
		"devops force": 33,
	}
	// Fetch issues for each project
	for title, projID := range projects {
		tmp, err := a.ghClient.GetOrgProject(ctx, a.ghOrg, projID, since)
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
	messages, err := q.FindTelegramMessages(ctx, pgtype.Timestamptz{Time: since, Valid: true})
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
