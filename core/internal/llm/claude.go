package llm

import (
	"context"
	"encoding/json"

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
	resp, err := c.ChatWithTools(ctx, systemPrompt, messages, nil)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (c *claude) ChatWithTools(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*ChatResponse, error) {
	anthropicMessages := c.convertMessages(messages)

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

	if len(tools) > 0 {
		params.Tools = c.convertTools(tools)
	}

	resp, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, err
	}

	return c.parseResponse(resp), nil
}

func (c *claude) convertMessages(messages []Message) []anthropic.MessageParam {
	var result []anthropic.MessageParam

	for _, msg := range messages {
		switch msg.Role {
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				var blocks []anthropic.ContentBlockParamUnion
				if msg.Content != "" {
					blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
				}
				for _, tc := range msg.ToolCalls {
					var input map[string]any
					json.Unmarshal([]byte(tc.Arguments), &input)
					blocks = append(blocks, anthropic.ContentBlockParamOfRequestToolUseBlock(tc.ID, input, tc.Name))
				}
				result = append(result, anthropic.NewAssistantMessage(blocks...))
			} else {
				result = append(result, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
			}
		case "tool":
			result = append(result, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false),
			))
		default:
			result = append(result, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		}
	}

	return result
}

func (c *claude) convertTools(tools []Tool) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, len(tools))

	for i, tool := range tools {
		schema := anthropic.ToolInputSchemaParam{
			Properties: tool.Parameters,
		}
		result[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: schema,
			},
		}
	}

	return result
}

func (c *claude) parseResponse(resp *anthropic.Message) *ChatResponse {
	result := &ChatResponse{
		StopReason: string(resp.StopReason),
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			result.Content = block.Text
		case "tool_use":
			args, _ := json.Marshal(block.Input)
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(args),
			})
		}
	}

	return result
}
