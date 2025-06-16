package llm

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"

	"mimi/internal/bot/llm/agent"
)

type LLM struct {
	g      *genkit.Genkit
	agents []agent.Agent
	router *ai.Prompt
}

func New() LLM {
	ctx := context.Background()
	g, err := genkit.Init(ctx,
		genkit.WithPlugins(&googlegenai.GoogleAI{}),
		genkit.WithDefaultModel("googleai/gemini-2.0-flash"),
	)
	if err != nil {
		log.Fatalf("could not initialize Genkit: %v", err)
	}

	agents := []agent.Agent{
		agent.NewLogseqAgent(g),
		agent.NewFallbackAgent(g),
	}

	router := genkit.LookupPrompt(g, "router")
	if router == nil {
		log.Fatal("no prompt named 'router' found")
	}

	return LLM{
		g:      g,
		agents: agents,
		router: router,
	}
}

func (m LLM) Answer(ctx context.Context, query string) (string, error) {
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
	return output.Agent, nil
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
