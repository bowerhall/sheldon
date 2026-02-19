package llm

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type claude struct {
	client anthropic.Client
	model  string
}

func newClaude(apiKey, model string) LLM {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &claude{client: client, model: model}
}

func (c *claude) Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error) {
	anthropicMessages := make([]anthropic.MessageParam, len(messages))

	for i, msg := range messages {
		if msg.Role == "assistant" {
			anthropicMessages[i] = anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content))
		} else {
			anthropicMessages[i] = anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 4096,
		Messages:  anthropicMessages,
	}

	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	resp, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return "", err
	}

	for _, block := range resp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}

	return "", nil
}
