package agent

import (
	"context"

	"github.com/firebase/genkit/go/ai"
)

type DataText struct {
	Text string
}

type DataFile struct {
	Blob []byte
	Name string
}

type Response struct {
	Data any
	Raw  *ai.ModelResponse
}

type DataType interface {
	DataText | DataFile
}

func NewResponse[T DataType](data T, raw *ai.ModelResponse) Response {
	return Response{
		Data: data,
		Raw:  raw,
	}
}

type Agent interface {
	GetInfo() Info
	Run(ctx context.Context, query string, msgs ...*ai.Message) (Response, error)
}

type Info struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
