package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"

	"github.com/cozodb/cozo-lib-go"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/bot/llm/agent"
	"mimi/internal/bot/llm/agent/fallback"
	"mimi/internal/bot/llm/agent/github"
	"mimi/internal/bot/llm/agent/logseq"
	"mimi/internal/bot/llm/agent/logseqquery"
	"mimi/internal/bot/llm/agent/summary"
	"mimi/internal/bot/llm/agent/telegram"
	"mimi/internal/persist"
	logseqscraper "mimi/internal/provider/logseq"
	"mimi/internal/provider/logseq/db"
)

type LLM struct {
	g      *genkit.Genkit
	q      *persist.Queries
	agents map[string]agent.Agent
	router *ai.Prompt
}

func New(pgPool *pgxpool.Pool, graph logseqscraper.RegexGraph, g *genkit.Genkit, conn cozo.CozoDB) LLM {
	q := persist.New(pgPool)

	ghOrg := "cyber-valley"
	agents := []agent.Agent{
		logseq.New(g, db.New(conn)),
		logseqquery.New(graph),
		fallback.New(g),
		github.New(g, ghOrg),
		telegram.New(g, pgPool),
		summary.New(g, pgPool, ghOrg, graph.Path),
	}
	mapped := make(map[string]agent.Agent, len(agents))
	for _, agent := range agents {
		mapped[agent.GetInfo().Name] = agent
	}

	router := genkit.LookupPrompt(g, "router")
	if router == nil {
		log.Fatal("no prompt named 'router' found")
	}

	return LLM{
		g:      g,
		q:      q,
		agents: mapped,
		router: router,
	}
}

func (m LLM) Answer(ctx context.Context, id int64, query string) (string, error) {
	// Route to the proper agent
	resp, err := m.router.Execute(ctx, ai.WithInput(map[string]any{
		"query":  query,
		"agents": m.getAgentsInfo(),
	}))
	if err != nil {
		return "", fmt.Errorf("initial LLM call failed with %w", err)
	}
	var output routerOutput
	if err := resp.Output(&output); err != nil {
		return "", fmt.Errorf("failed to parse router output with %w", err)
	}
	slog.Info("router answer", "agent", output.Agent)

	// Retrieve messages history
	rows, err := m.q.FindChatMessages(ctx, id)
	var messages []*ai.Message
	switch err {
	case pgx.ErrNoRows:
		break
	case nil:
		err = json.Unmarshal(rows, &messages)
		if err != nil {
			return "", fmt.Errorf("failed to unmarshal messages with %w", err)
		}
	default:
		return "", fmt.Errorf("failed to find message history with %w", err)
	}

	// Run selected agent
	a, ok := m.agents[output.Agent]
	if !ok {
		return "", fmt.Errorf("agent with name '%s' not found", output.Agent)
	}
	resp, err = a.Run(ctx, query, messages...)
	if err != nil {
		return "", fmt.Errorf("failed to run agent with %w", err)
	}

	// Update message history
	messages = append(messages, ai.NewTextMessage(ai.RoleUser, query))
	messages = append(messages, resp.Message)
	encoded, err := json.Marshal(messages)
	if err != nil {
		return "", fmt.Errorf("failed to marshal messages with %w", err)
	}
	err = m.q.SaveChatMessages(ctx, persist.SaveChatMessagesParams{
		TelegramID: id,
		Messages:   encoded,
	})
	if err != nil {
		return "", fmt.Errorf("failed to save messages with %w", err)
	}

	return resp.Text(), nil
}

func (m LLM) getAgentsInfo() (info []agent.Info) {
	for _, agent := range m.agents {
		info = append(info, agent.GetInfo())
	}
	return info
}

type routerOutput struct {
	Agent string `json:"agent"`
}
