package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/bot/llm/agent"
	"mimi/internal/persist"
)

const (
	retrievePrompt        = "telegram-retrieve"
	evalPrompt            = "telegram-eval"
	telegramSchemaPath    = "sql/migrations/000001_init.up.sql"
	thinkingMaxItarations = 5
)

type TelegramAgent struct {
	pgPool         *pgxpool.Pool
	retrievePrompt *ai.Prompt
	evalPrompt     *ai.Prompt
	sqlSchema      string
}

func New(g *genkit.Genkit, pgPool *pgxpool.Pool) TelegramAgent {
	// Fail fast if prompt wasn't found
	retrieve := genkit.LookupPrompt(g, retrievePrompt)
	if retrieve == nil {
		log.Fatalf("no prompt named '%s' found", retrievePrompt)
	}
	eval := genkit.LookupPrompt(g, evalPrompt)
	if eval == nil {
		log.Fatalf("no prompt named '%s' found", evalPrompt)
	}

	// Define a SQL query tool
	genkit.DefineTool(
		g, "queryDB", "Executes given PostgreSQL query and returns results",
		func(ctx *ai.ToolContext, input sqlQuery) (string, error) {
			// Execute query
			rows, err := pgPool.Query(ctx, input.SQL)
			if err != nil {
				return "", fmt.Errorf("failed to execute generated SQL query '%s' with %w", input.SQL, err)
			}
			defer rows.Close()

			// Scan rows
			var data [][]any
			for rows.Next() {
				row, err := rows.Values()
				if err != nil {
					return "", fmt.Errorf("failed to scan row with %w", err)
				}
				data = append(data, row)
			}
			slog.Info("retrieved rows from generated SQL query", "query", input.SQL, "length", len(data))

			// Serialize into JSON
			blob, err := json.Marshal(data)
			if err != nil {
				return "", fmt.Errorf("failed to serialzie collected rows from '%s' into JSON with %w", input.SQL, err)
			}
			return string(blob), nil
		})

	// Read telegram Schema
	schema, err := os.ReadFile(telegramSchemaPath)
	if err != nil {
		log.Fatalf("failed to read schema from %s with %s", telegramSchemaPath, err)
	}
	slog.Info("read SQL schema", "value", schema)

	return TelegramAgent{
		pgPool:         pgPool,
		retrievePrompt: retrieve,
		evalPrompt:     eval,
		sqlSchema:      string(schema),
	}
}

type sqlQuery struct {
	SQL string `json:"sql" jsonschema_description:"Query to execute"`
}

func (a TelegramAgent) GetInfo() agent.Info {
	return agent.Info{
		Name:        "telegram",
		Description: `Has access to telegram message and capable of providing summaries or followbacks about current devops force or rockets live statuses`,
	}
}

func (a TelegramAgent) Run(ctx context.Context, query string, msgs ...*ai.Message) (*ai.ModelResponse, error) {
	q := persist.New(a.pgPool)
	info, err := q.FindTelegramPeersWithTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch initial info for telegram agent run with %w", err)
	}
	blob, err := json.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal telegram info with %w", err)
	}
	// Retrieve related DB info
	resp, err := a.retrievePrompt.Execute(
		ctx,
		ai.WithDocs(ai.DocumentFromText(string(blob), map[string]any{"info": "current telegram chats and topics"})),
		ai.WithMessages(msgs...),
		ai.WithInput(map[string]any{"query": query, "schema": a.sqlSchema}),
	)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed with %w", err)
	}

	slog.Info("retrieved telegram data", "value", resp.Text())

	// Generate response based on fetched rows
	resp, err = a.evalPrompt.Execute(
		ctx,
		ai.WithMessages(msgs...),
		ai.WithDocs(ai.DocumentFromText(resp.Text(), map[string]any{})),
		ai.WithInput(map[string]any{"query": query}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate final step with %w", err)
	}

	return resp, nil
}
