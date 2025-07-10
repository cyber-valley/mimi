package fallback

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"mimi/internal/bot/llm/agent"
)

type FallbackAgent struct {
	g *genkit.Genkit
}

func New(g *genkit.Genkit) FallbackAgent {
	type FallbackInput struct {
		Query string `json:"query" jsonschema_description:"User's original query"`
	}
	ag := FallbackAgent{g: g}
	genkit.DefineTool(
		g, "fallback", "Should be used if there is not enough context info to answer user's query",
		func(ctx *ai.ToolContext, input FallbackInput) (string, error) {
			slog.Info("call to fallback tool")
			resp, err := ag.Run(ctx, input.Query)
			if err != nil {
				return "", err
			}
			return resp.Text(), nil
		})
	return ag
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
		ai.WithModelName("openai/perplexity/sonar-pro"),
		ai.WithMessages(msgs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call fallback agent with %w", err)
	}
	return resp, nil
}
