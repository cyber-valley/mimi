package llm

import (
	"context"
	"fmt"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"

	"mimi/internal/bot/llm/agent"
)

type LLM struct {
	g      *genkit.Genkit
	agents map[string]agent.Agent
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
	nameToAgent := make(map[string]agent.Agent)
	for _, a := range agents {
		nameToAgent[a.GetInfo().Name] = a
	}

	return LLM{
		g:      g,
		agents: nameToAgent,
	}
}

func (m LLM) Answer(ctx context.Context, query string) (string, error) {
	resp, err := genkit.Generate(ctx, m.g, ai.WithPrompt(query))
	if err != nil {
		return "", fmt.Errorf("initial LLM call failed with %w", err)
	}
	return resp.Text(), nil
}
