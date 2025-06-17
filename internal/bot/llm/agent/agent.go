package agent

import (
	"context"

	"github.com/firebase/genkit/go/ai"
)

type Agent interface {
	GetInfo() Info
	Run(ctx context.Context, query string, msgs ...*ai.Message) (*ai.ModelResponse, error)
}

type Info struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
