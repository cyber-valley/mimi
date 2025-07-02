package fallback

import (
	"context"
	"fmt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"mimi/internal/bot/llm/agent"
)

type FallbackAgent struct {
	g *genkit.Genkit
}

func New(g *genkit.Genkit) FallbackAgent {
	return FallbackAgent{g: g}
}

func (a FallbackAgent) GetInfo() agent.Info {
	return agent.Info{
		Name:        "fallback",
		Description: `used if there is no any better option`,
	}
}

func (a FallbackAgent) Run(ctx context.Context, query string, msgs ...*ai.Message) (*ai.ModelResponse, error) {
	resp, err := genkit.Generate(
		ctx,
		a.g,
		ai.WithPrompt(query),
		ai.WithMessages(msgs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call fallback agent with %w", err)
	}
	return resp, nil
}
