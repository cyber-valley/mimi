package agent

import (
	"context"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

const (
	evalSummaryPrompt = "summary"
)

type SummaryAgent struct {
	evalPrompt     *ai.Prompt
}

func NewSummaryAgent(g *genkit.Genkit, logseqRepoPath string, tgAgent, ghAgent Agent) SummaryAgent {
	// Fail fast if prompt wasn't found
	eval := genkit.LookupPrompt(g, evalSummaryPrompt)
	if eval == nil {
		log.Fatalf("no prompt named '%s' found", evalSummaryPrompt)
	}

	// Define tools
  genkit.DefineTool(
    g, "logseqDiff", "Returns `git diff` from the given date to the latest commit",
    func(ctx *ai.ToolContext, input logseqDiffInput) (string, error) {
			return fetchLogseqDiff(ctx, logseqRepoPath, input)
		})

  genkit.DefineTool(
    g, "githubAgent", "Natural language interface to access GitHub projects",
    func(ctx *ai.ToolContext, input string) (string, error) {
			resp, err := tgAgent.Run(ctx, input)
			if err != nil {
				return "", err
			}
			return resp.Text(), nil
		})

  genkit.DefineTool(
    g, "telegramAgent", "Natural language interface to access Telegram chats",
    func(ctx *ai.ToolContext, input string) (string, error) {
			resp, err := ghAgent.Run(ctx, input)
			if err != nil {
				return "", err
			}
			return resp.Text(), nil
		})

	return SummaryAgent{
		evalPrompt:     eval,
	}
}

func (a SummaryAgent) GetInfo() Info {
	return Info{
		Name: "summary",
		Description: `Provides overall summary across all available resources`,
	}
}

func (a SummaryAgent) Run(ctx context.Context, query string, msgs ...*ai.Message) (*ai.ModelResponse, error) {
	return a.evalPrompt.Execute(ctx, ai.WithInput(map[string]any{"query": query}))
}

type logseqDiffInput struct {
	Period string `json:"period" jsonschema_description:"day, week, or month"`
}

func fetchLogseqDiff(ctx *ai.ToolContext, logseqRepoPah string, input logseqDiffInput) (string, error) {
	return "not implemented", nil
}
