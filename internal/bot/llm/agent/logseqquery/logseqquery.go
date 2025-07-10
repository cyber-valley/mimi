package logseqquery

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"

	"github.com/firebase/genkit/go/ai"

	"mimi/internal/bot/llm/agent"
	"mimi/internal/provider/logseq"
	"mimi/internal/provider/logseq/query"
)

type LogseqQueryAgent struct {
	graph logseq.RegexGraph
}

func New(graph logseq.RegexGraph) LogseqQueryAgent {
	return LogseqQueryAgent{graph: graph}
}

func (a LogseqQueryAgent) GetInfo() agent.Info {
	return agent.Info{
		Name: "logseq-query",
		Description: `Evaluates logseq queries of format like '{{query (property :supply "next-month")}}'. They can have additional parameters like '{{query (page-tags [[super]])}}
  query-properties:: [:page :tags :alias]
  query-sort-by:: page
	query-sort-desc:: true'

		As a result it returns a CSV file with results`,
	}
}

func (a LogseqQueryAgent) Run(ctx context.Context, queryS string, msgs ...*ai.Message) (*ai.ModelResponse, error) {
	slog.Info("trying to eval logseq query")
	result, err := query.Eval(ctx, a.graph, queryS)
	if err != nil {
		slog.Warn("failed to evaluate logseq query", "with", err)
		return nil, fmt.Errorf("failed to evaluate query with %w", err)
	}
	slog.Info("got logseq query result", "rows", len(result.Table)-1)

	// Encode as CSV
	buf := new(bytes.Buffer)
	writer := csv.NewWriter(buf)
	err = writer.WriteAll(result.Table)
	if err != nil {
		return nil, fmt.Errorf("failed to encode LogSeq query results into CSV with %w", err)
	}
	return &ai.ModelResponse{
		Message: ai.NewTextMessage(ai.RoleModel, buf.String()),
	}, nil
}
