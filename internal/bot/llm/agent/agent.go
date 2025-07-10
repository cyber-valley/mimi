package agent

import (
	"context"
	"fmt"

	"github.com/firebase/genkit/go/ai"
)

var ErrEmptyContext = fmt.Errorf("required context is empty")

type Agent interface {
	GetInfo() Info
	Run(ctx context.Context, query string, msgs ...*ai.Message) (*ai.ModelResponse, error)
}

type Info struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
