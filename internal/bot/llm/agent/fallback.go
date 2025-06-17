package agent

import (
	"context"
	"fmt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

type FallbackAgent struct {
	g *genkit.Genkit
}

func NewFallbackAgent(g *genkit.Genkit) FallbackAgent {
	return FallbackAgent{g: g}
}

func (a FallbackAgent) GetInfo() Info {
	return Info{
		Name:        "fallback",
		Description: `used if there is no any better option`,
	}
}

func (a FallbackAgent) Run(ctx context.Context, query string) (string, error) {
	resp, err := genkit.Generate(ctx, a.g, ai.WithPrompt(query))
	if err != nil {
		return "", fmt.Errorf("failed to call fallback agent with %w", err)
	}
	return resp.Text(), nil
}
