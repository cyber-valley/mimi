package agent

import (
	"context"

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
		Name: "fallback",
		Description: `used if there is no any better option`,
	}
}

func (a FallbackAgent) Run(ctx context.Context, query string) (string, error) {
	panic("not implemented")
}

