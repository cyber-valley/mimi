package fallback

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"mimi/internal/bot/llm/agent"
)

const (
	evalPromptName = "fallback"
)

type FallbackAgent struct {
	g          *genkit.Genkit
	evalPrompt *ai.Prompt
}

type fallbackInput struct {
	Query string `json:"query" jsonschema_description:"User's original query"`
}

func New(g *genkit.Genkit) FallbackAgent {
	// Fail fast if prompt wasn't found
	evalPrompt := genkit.LookupPrompt(g, evalPromptName)
	if evalPrompt == nil {
		log.Fatalf("no prompt named '%s' found", evalPromptName)
	}
	ag := FallbackAgent{g: g, evalPrompt: evalPrompt}

	genkit.DefineTool(
		g, "fallback", "Should be used if there is not enough context info to answer user's query",
		func(ctx *ai.ToolContext, input fallbackInput) (string, error) {
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
	resp, err := a.evalPrompt.Execute(
		ctx,
		ai.WithInput(fallbackInput{Query: query}),
		ai.WithMessages(msgs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call fallback agent with %w", err)
	}
	return resp, nil
}
