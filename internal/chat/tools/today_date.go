package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/openai/openai-go/v2"
)

type TodayDateTool struct{}

func (w TodayDateTool) Name() string        { return "get_today_date" }
func (w TodayDateTool) Description() string { return "Get today's date and time in RFC3339 format" }
func (w TodayDateTool) Parameters() openai.FunctionParameters {
	return nil
}

func (w TodayDateTool) Handle(ctx context.Context, args json.RawMessage) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}

func init() {
	Register(TodayDateTool{})
}
