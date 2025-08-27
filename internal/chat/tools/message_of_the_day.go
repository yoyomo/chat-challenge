package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/openai/openai-go/v2"
)

type MessageOfTheDayTool struct{}

func (m MessageOfTheDayTool) Name() string        { return "get_message_of_the_day" }
func (m MessageOfTheDayTool) Description() string { return "Returns the message of the day" }
func (m MessageOfTheDayTool) Parameters() openai.FunctionParameters {
	return nil
}
func (m MessageOfTheDayTool) Handle(ctx context.Context, args json.RawMessage) (string, error) {
	resp, err := http.Get("https://zenquotes.io/api/today")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get message of the day: %s", resp.Status)
	}

	var quote struct {
		Quote string `json:"q"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return "", err
	}

	return quote.Quote, nil
}

func init() {
	Register(MessageOfTheDayTool{})
}
