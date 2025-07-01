package agent

import (
	"context"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/jackc/pgx/v5"
)

const (
	evalSummaryPrompt = "summary"
)

type SummaryAgent struct {
	conn              *pgx.Conn
	evalPrompt     *ai.Prompt
}

func NewSummaryAgent(g *genkit.Genkit, conn *pgx.Conn) SummaryAgent {
	// Fail fast if prompt wasn't found
	eval := genkit.LookupPrompt(g, evalSummaryPrompt)
	if eval == nil {
		log.Fatalf("no prompt named '%s' found", evalSummaryPrompt)
	}

	// Define tools
  genkit.DefineTool(
    g, "logseqDiff", "Returns `git diff` from the given date to the latest commit",
    func(ctx *ai.ToolContext, input any) (string, error) {
			panic("not implemented")
		})

  genkit.DefineTool(
    g, "githubAgent", "Natural language interface to access GitHub projects",
    func(ctx *ai.ToolContext, input any) (string, error) {
			panic("not implemented")
		})

  genkit.DefineTool(
    g, "telegramAgent", "Natural language interface to access Telegram chats",
    func(ctx *ai.ToolContext, input any) (string, error) {
			panic("not implemented")
		})

	return SummaryAgent{
		conn:              conn,
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
	panic("not implemented")
}

