package agent

import (
	"context"
)

type Agent interface {
	GetInfo() Info
	Run(ctx context.Context, query string) (string, error)
}

type Info struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
