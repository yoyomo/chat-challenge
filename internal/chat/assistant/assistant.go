package assistant

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/acai-travel/tech-challenge/internal/chat/tools"
	"github.com/openai/openai-go/v2"
)

type Assistant struct {
	cli openai.Client
}

func New() *Assistant {
	return &Assistant{cli: openai.NewClient()}
}

func (a *Assistant) Title(ctx context.Context, conv *model.Conversation) (string, error) {
	if len(conv.Messages) == 0 {
		return "An empty conversation", nil
	}

	slog.InfoContext(ctx, "Generating title for conversation", "conversation_id", conv.ID)

	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("Summarize the user's question as a concise, descriptive title. The title should be a single line, no more than 80 characters, and should not include any special characters or emojis."),
	}
	for _, m := range conv.Messages {
		if m.Role == model.RoleUser {
			msgs = append(msgs, openai.UserMessage(m.Content))
		}
	}

	resp, err := a.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    openai.ChatModelO1,
		Messages: msgs,
	})

	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return "", errors.New("empty response from OpenAI for title generation")
	}

	title := resp.Choices[0].Message.Content
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.Trim(title, " \t\r\n-\"'")

	if len(title) > 80 {
		title = title[:80]
	}

	return title, nil
}

func (a *Assistant) Reply(ctx context.Context, conv *model.Conversation) (string, error) {
	if len(conv.Messages) == 0 {
		return "", errors.New("conversation has no messages")
	}

	slog.InfoContext(ctx, "Generating reply for conversation", "conversation_id", conv.ID)

	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful, concise AI assistant. Provide accurate, safe, and clear responses."),
	}

	for _, m := range conv.Messages {
		switch m.Role {
		case model.RoleUser:
			msgs = append(msgs, openai.UserMessage(m.Content))
		case model.RoleAssistant:
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		}
	}

	for i := 0; i < 15; i++ {
		toolDefs := []openai.ChatCompletionToolUnionParam{}
		for _, tool := range tools.Registry {
			toolDefs = append(toolDefs, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
				Name:        tool.Name(),
				Description: openai.String(tool.Description()),
				Parameters:  tool.Parameters(),
			}))
		}
		resp, err := a.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    openai.ChatModelGPT4_1,
			Messages: msgs,
			Tools:    toolDefs,
		})

		if err != nil {
			return "", err
		}

		if len(resp.Choices) == 0 {
			return "", errors.New("no choices returned by OpenAI")
		}

		if message := resp.Choices[0].Message; len(message.ToolCalls) > 0 {
			msgs = append(msgs, message.ToParam())

			for _, call := range message.ToolCalls {
				slog.InfoContext(ctx, "Tool call received", "name", call.Function.Name, "args", call.Function.Arguments)
				tool, ok := tools.Registry[call.Function.Name]
				if !ok {
					return "", errors.New("unknown tool call: " + call.Function.Name)
				}
				result, err := tool.Handle(ctx, []byte(call.Function.Arguments))
				if err != nil {
					msgs = append(msgs, openai.ToolMessage(err.Error(), call.ID))
					continue
				}
				msgs = append(msgs, openai.ToolMessage(result, call.ID))

			}

			continue
		}

		return resp.Choices[0].Message.Content, nil
	}

	return "", errors.New("too many tool calls, unable to generate reply")
}
