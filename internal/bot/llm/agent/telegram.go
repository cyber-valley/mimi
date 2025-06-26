package agent

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"encoding/json"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/jackc/pgx/v5"
)

const (
	retrievePromptName = "telegram-retrieve"
	evalPromptName = "telegram-eval"
	telegramSchemaPath = "sql/migrations/000001_telegram.up.sql"
)

type TelegramAgent struct {
	conn              *pgx.Conn
	retrievePrompt *ai.Prompt
	evalPrompt     *ai.Prompt
	sqlSchema string
}

func NewTelegramAgent(g *genkit.Genkit, conn *pgx.Conn) TelegramAgent {
	// Fail fast if prompt wasn't found
	retrieve := genkit.LookupPrompt(g, retrievePromptName)
	if retrieve == nil {
		log.Fatalf("no prompt named '%s' found", retrievePromptName)
	}
	eval := genkit.LookupPrompt(g, evalPromptName)
	if eval == nil {
		log.Fatalf("no prompt named '%s' found", evalPromptName)
	}

	// Read telegram Schema
	schema, err := os.ReadFile(telegramSchemaPath)
	if err != nil {
		log.Fatalf("failed to read schema from %s with %s", telegramSchemaPath, err)
	}

	return TelegramAgent{
		conn:              conn,
		retrievePrompt: retrieve,
		evalPrompt:     eval,
		sqlSchema: string(schema),
	}
}

func (a TelegramAgent) GetInfo() Info {
	return Info{
		Name: "telegram",
		Description: `Has access to telegram message and capable of providing summaries or followbacks`,
	}
}

type sqlQuery struct {
	sql string
}

func (a TelegramAgent) Run(ctx context.Context, query string, msgs ...*ai.Message) (*ai.ModelResponse, error) {
	// Generate SQL query for the request
	resp, err := a.retrievePrompt.Execute(
		ctx,
		ai.WithMessages(msgs...),
		ai.WithInput(map[string]any{"query": query, "schema": a.sqlSchema}),
	)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed with %w", err)
	}

	// Execute generated qieru
	var q sqlQuery
	if err := resp.Output(&q); err != nil {
		return nil, fmt.Errorf("failed to parse LLM output with %w", err)
	}
	rows, err := a.conn.Query(ctx, q.sql)
	if err != nil {
		return nil, fmt.Errorf("failed to execute generated SQL query '%s' with %w", q.sql, err)
	}
	data, err := pgx.CollectRows(rows, pgx.RowTo[[]any])
	if err != nil {
		return nil, fmt.Errorf("failed to collect rows from generated SQL query '%s' with %w", q.sql, err)
	}
	slog.Info("retrieved rows from generated SQL query", "query", q.sql, "length", len(data))

	// Serialize into JSON
	blob, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to serialzie collected rows from '%s' into JSON with %w", q.sql, err)
	}

	// Generate response based on fetched rows
	resp, err = a.evalPrompt.Execute(
		ctx,
		ai.WithDocs(ai.DocumentFromText(string(blob), map[string]any{})),
		ai.WithInput(map[string]any{"query": query}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate final step with %w", err)
	}

	return resp, nil
}

