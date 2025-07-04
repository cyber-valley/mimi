package summary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/bot/llm/agent"
	"mimi/internal/persist"
	"mimi/internal/provider/git"
	"mimi/internal/provider/github/db"
)

const (
	evalPrompt   = "summary"
	periodPrompt = "period-extractor"
)

var githubProjects = map[string]int{
	"rockets":      2,
	"supply":       3,
	"inventory":    24,
	"devops force": 33,
}

type SummaryAgent struct {
	evalPrompt      *ai.Prompt
	periodExtractor *ai.Prompt
	ghClient        *db.Client
	ghOrg           string
	pgPool          *pgxpool.Pool
	logseqRepoPath  string
}

func New(g *genkit.Genkit, pgPool *pgxpool.Pool, ghOrg, logseqRepoPath string) SummaryAgent {
	// Fail fast if prompt wasn't found
	eval := genkit.LookupPrompt(g, evalPrompt)
	if eval == nil {
		log.Fatalf("no prompt named '%s' found", evalPrompt)
	}

	periodExtractor := genkit.LookupPrompt(g, periodPrompt)
	if periodExtractor == nil {
		log.Fatalf("no prompt named '%s' found", periodPrompt)
	}

	return SummaryAgent{
		pgPool:          pgPool,
		ghClient:        db.New("https://api.github.com/graphql"),
		ghOrg:           ghOrg,
		evalPrompt:      eval,
		periodExtractor: periodExtractor,
		logseqRepoPath:  logseqRepoPath,
	}
}

func (a SummaryAgent) GetInfo() agent.Info {
	return agent.Info{
		Name:        "summary",
		Description: `Provides overall summary across all available resources`,
	}
}

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

	docChan := make(chan *ai.Document, 3)
	errChan := make(chan error, 3)
	var wg sync.WaitGroup
	wg.Add(3)
	startT := time.Now()

	// Retrieve GitHub projects statuses
	go func() {
		defer wg.Done()
		type msg struct {
			project string
			issues  []db.Issue
		}

		var projWg sync.WaitGroup
		issueChan := make(chan msg, len(githubProjects))

		// Fetch issues for each project
		for title, projID := range githubProjects {
			projWg.Add(1)
			go func() {
				defer projWg.Done()
				tmp, err := a.ghClient.GetOrgProject(ctx, a.ghOrg, projID, since)
				if err != nil {
					errChan <- fmt.Errorf("failed to fetch supply board state with %w", err)
					return
				}
				slog.Info("fetched GitHub issues", "project", title, "lenght", len(tmp), "value", tmp)
				issueChan <- msg{project: title, issues: tmp}
			}()
		}

		// Wait for the fetched issues
		projWg.Wait()
		close(issueChan)

		// Send retrieved project issues
		issues := make(map[string][]db.Issue)
		for msg := range issueChan {
			issues[msg.project] = msg.issues
		}
		blob, err := json.Marshal(issues)
		if err != nil {
			errChan <- fmt.Errorf("failed to marshal GitHub projects info with %w", err)
			return
		}
		docChan <- ai.DocumentFromText(string(blob), map[string]any{"info": "GitHub projects issues"})
	}()

	// Retrieve Telegram info
	go func() {
		defer wg.Done()
		q := persist.New(a.pgPool)
		messages, err := q.FindTelegramMessages(ctx, pgtype.Timestamptz{Time: since, Valid: true})
		if err != nil {
			errChan <- fmt.Errorf("failed to retrieve Telegram message from DB with %w", err)
			return
		}
		slog.Info("retrieved Telegram messages", "length", len(messages))
		blob, err := json.Marshal(messages)
		if err != nil {
			errChan <- fmt.Errorf("failed to marshal Telegram messages with %w", err)
			return
		}
		docChan <- ai.DocumentFromText(string(blob), map[string]any{"info": "Related telegram messages"})
	}()

	// Retrieve LogSeq diff
	go func() {
		defer wg.Done()
		diff, err := git.DiffInterval(a.logseqRepoPath, since)
		if err != nil {
			errChan <- err
			return
		}
		slog.Info("retrieved LogSeq diff", "length", len(diff))
		docChan <- ai.DocumentFromText(diff, map[string]any{"info": "LogSeq git diff"})
	}()

	wg.Wait()
	slog.Info("summary data retrieved", "elapsed", time.Since(startT))
	close(errChan)
	close(docChan)

	// Process errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to retrieve data for summary with %w", errors.Join(errs...))
	}

	// Call LLM
	var docs []*ai.Document
	for doc := range docChan {
		docs = append(docs, doc)
	}

	resp, err = a.evalPrompt.Execute(ctx, ai.WithDocs(docs...), ai.WithInput(map[string]any{"period": period}))
	if err != nil {
		return nil, err
	}
	slog.Info("generated summary", "text", resp.Text())
	return resp, nil
}
