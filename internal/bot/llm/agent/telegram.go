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
	thinkingMaxIterations int
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

	// Define a SQL query tool
  genkit.DefineTool(
    g, "queryDB", "Executes given PostgreSQL query and returns results",
    func(ctx *ai.ToolContext, input sqlQuery) (string, error) {
			rows, err := conn.Query(ctx, input.SQL)
			if err != nil {
				return "", fmt.Errorf("failed to execute generated SQL query '%s' with %w", input.SQL, err)
			}
			data, err := pgx.CollectRows(rows, pgx.RowTo[[]any])
			if err != nil {
				return "", fmt.Errorf("failed to collect rows from generated SQL query '%s' with %w", input.SQL, err)
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

	return TelegramAgent{
		conn:              conn,
		retrievePrompt: retrieve,
		evalPrompt:     eval,
		sqlSchema: string(schema),
		thinkingMaxIterations: 5,
	}
}

type sqlQuery struct {
	SQL string `json:"sql" jsonschema_description:"Query to execute"`
}

func (a TelegramAgent) GetInfo() Info {
	return Info{
		Name: "telegram",
		Description: `Has access to telegram message and capable of providing summaries or followbacks about current devops force or rockets live statuses`,
	}
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

	// Execute generated query
	var data telegramData
	if err := resp.Output(&data); err != nil {
		return nil, fmt.Errorf("failed to parse LLM output with %w", err)
	}

	// Generate response based on fetched rows
	resp, err = a.evalPrompt.Execute(
		ctx,
		ai.WithDocs(ai.DocumentFromText(data.Data, map[string]any{})),
		ai.WithInput(map[string]any{"query": query}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate final step with %w", err)
	}

	return resp, nil
}

type telegramData struct {
	Data string `json:"data"`
}

