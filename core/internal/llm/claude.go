package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"regexp"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// validToolIDPattern matches Claude's required pattern for tool IDs
var validToolIDPattern = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

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
					// sanitize tool ID to match Claude's required pattern
					toolID := sanitizeToolID(tc.ID)
					blocks = append(blocks, anthropic.ContentBlockParamOfRequestToolUseBlock(toolID, input, tc.Name))
				}
				result = append(result, anthropic.NewAssistantMessage(blocks...))
			} else {
				result = append(result, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
			}
		case "tool":
			// sanitize tool ID to match Claude's required pattern
			toolID := sanitizeToolID(msg.ToolCallID)
			result = append(result, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(toolID, msg.Content, false),
			))
		default:
			var blocks []anthropic.ContentBlockParamUnion

			for _, img := range msg.Images {
				blocks = append(blocks, anthropic.NewImageBlockBase64(
					img.MediaType,
					base64.StdEncoding.EncodeToString(img.Data),
				))
			}

			if msg.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}

			if len(blocks) > 0 {
				result = append(result, anthropic.NewUserMessage(blocks...))
			}
		}
	}

	return result
}

// sanitizeToolID ensures tool IDs match Claude's pattern ^[a-zA-Z0-9_-]+$
func sanitizeToolID(id string) string {
	return validToolIDPattern.ReplaceAllString(id, "_")
}

func (c *claude) convertTools(tools []Tool) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, len(tools))

	for i, tool := range tools {
		// extract properties and required from the full schema
		props := make(map[string]any)
		var required []string

		if p, ok := tool.Parameters["properties"].(map[string]any); ok {
			props = p
		}
		if r, ok := tool.Parameters["required"].([]string); ok {
			required = r
		} else if r, ok := tool.Parameters["required"].([]any); ok {
			for _, v := range r {
				if s, ok := v.(string); ok {
					required = append(required, s)
				}
			}
		}

		schema := anthropic.ToolInputSchemaParam{
			Properties: props,
		}
		if len(required) > 0 {
			schema.ExtraFields = map[string]any{"required": required}
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
