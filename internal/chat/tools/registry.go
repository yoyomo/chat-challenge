package tools

import (
	"context"
	"encoding/json"

	"github.com/openai/openai-go/v2"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() openai.FunctionParameters
	Handle(ctx context.Context, args json.RawMessage) (string, error)
}

var Registry = map[string]Tool{}

func Register(tool Tool) {
	Registry[tool.Name()] = tool
}
