package agent

import (
	"context"

	"github.com/firebase/genkit/go/genkit"
)

type LogseqAgent struct {
	g *genkit.Genkit
}

func NewLogseqAgent(g *genkit.Genkit) LogseqAgent {
	return LogseqAgent{g: g}
}

func (a LogseqAgent) GetInfo() Info {
	return Info{
		Name: "logseq",
		Description: `Knows all about cyber valley's history.
		Capable of answering to questions about flora and fauna, main goals and mindsets`,
	}
}

func (a LogseqAgent) Run(ctx context.Context, query string) (string, error) {
	panic("not implemented")
}
