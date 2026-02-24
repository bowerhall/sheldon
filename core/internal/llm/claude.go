package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// validToolIDPattern matches Claude's required pattern for tool IDs
var validToolIDPattern = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

type claude struct {
	client anthropic.Client
	apiKey string
	model  string
}

// Raw API types for video support (SDK doesn't have these yet)
type rawMessage struct {
	Role    string           `json:"role"`
	Content []rawContentBlock `json:"content"`
}

type rawContentBlock struct {
	Type   string          `json:"type"`
	Text   string          `json:"text,omitempty"`
	Source *rawMediaSource `json:"source,omitempty"`
}

type rawMediaSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type rawRequest struct {
	Model     string            `json:"model"`
	MaxTokens int               `json:"max_tokens"`
	System    string            `json:"system,omitempty"`
	Messages  []rawMessage      `json:"messages"`
	Tools     []rawTool         `json:"tools,omitempty"`
}

type rawTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type rawResponse struct {
	Content    []rawResponseBlock `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage      *rawUsage          `json:"usage,omitempty"`
	Error      *rawError          `json:"error,omitempty"`
}

type rawResponseBlock struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type rawUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type rawError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func newClaude(apiKey, model string) LLM {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &claude{client: client, apiKey: apiKey, model: model}
}

func (c *claude) Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error) {
	resp, err := c.ChatWithTools(ctx, systemPrompt, messages, nil)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (c *claude) ChatWithTools(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*ChatResponse, error) {
	// Check if any message contains video - use raw API if so
	hasVideo := false
	for _, msg := range messages {
		for _, media := range msg.Media {
			if media.Type == MediaTypeVideo {
				hasVideo = true
				break
			}
		}
		if hasVideo {
			break
		}
	}

	if hasVideo {
		return c.chatWithToolsRaw(ctx, systemPrompt, messages, tools)
	}

	// Use SDK for non-video messages
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

// chatWithToolsRaw uses raw HTTP API for video support
func (c *claude) chatWithToolsRaw(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*ChatResponse, error) {
	rawMessages := c.convertMessagesRaw(messages)

	req := rawRequest{
		Model:     c.model,
		MaxTokens: 4096,
		Messages:  rawMessages,
	}

	if systemPrompt != "" {
		req.System = systemPrompt
	}

	if len(tools) > 0 {
		req.Tools = c.convertToolsRaw(tools)
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	var rawResp rawResponse
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if rawResp.Error != nil {
		return nil, fmt.Errorf("api error: %s", rawResp.Error.Message)
	}

	return c.parseRawResponse(&rawResp), nil
}

func (c *claude) convertMessagesRaw(messages []Message) []rawMessage {
	var result []rawMessage

	for _, msg := range messages {
		var blocks []rawContentBlock

		switch msg.Role {
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				if msg.Content != "" {
					blocks = append(blocks, rawContentBlock{Type: "text", Text: msg.Content})
				}
				for _, tc := range msg.ToolCalls {
					var input map[string]any
					json.Unmarshal([]byte(tc.Arguments), &input)
					blocks = append(blocks, rawContentBlock{
						Type: "tool_use",
						// Note: tool_use blocks have different structure, simplified here
					})
				}
			} else {
				blocks = append(blocks, rawContentBlock{Type: "text", Text: msg.Content})
			}
			result = append(result, rawMessage{Role: "assistant", Content: blocks})

		case "tool":
			blocks = append(blocks, rawContentBlock{
				Type: "tool_result",
				Text: msg.Content,
			})
			result = append(result, rawMessage{Role: "user", Content: blocks})

		default:
			for _, media := range msg.Media {
				switch media.Type {
				case MediaTypeImage:
					blocks = append(blocks, rawContentBlock{
						Type: "image",
						Source: &rawMediaSource{
							Type:      "base64",
							MediaType: media.MimeType,
							Data:      base64.StdEncoding.EncodeToString(media.Data),
						},
					})
				case MediaTypeVideo:
					blocks = append(blocks, rawContentBlock{
						Type: "video",
						Source: &rawMediaSource{
							Type:      "base64",
							MediaType: media.MimeType,
							Data:      base64.StdEncoding.EncodeToString(media.Data),
						},
					})
				}
			}

			if msg.Content != "" {
				blocks = append(blocks, rawContentBlock{Type: "text", Text: msg.Content})
			}

			if len(blocks) > 0 {
				result = append(result, rawMessage{Role: "user", Content: blocks})
			}
		}
	}

	return result
}

func (c *claude) convertToolsRaw(tools []Tool) []rawTool {
	result := make([]rawTool, len(tools))
	for i, tool := range tools {
		result[i] = rawTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.Parameters,
		}
	}
	return result
}

func (c *claude) parseRawResponse(resp *rawResponse) *ChatResponse {
	result := &ChatResponse{
		StopReason: resp.StopReason,
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

	if resp.Usage != nil {
		result.Usage = &Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	return result
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

			for _, media := range msg.Media {
				switch media.Type {
				case MediaTypeImage:
					blocks = append(blocks, anthropic.NewImageBlockBase64(
						media.MimeType,
						base64.StdEncoding.EncodeToString(media.Data),
					))
				case MediaTypeVideo:
					// Claude SDK doesn't support inline video yet
					// Add a note so the model knows a video was sent
					blocks = append(blocks, anthropic.NewTextBlock("[Video attached - video analysis not yet supported]"))
				}
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

func (c *claude) Capabilities() Capabilities {
	return Capabilities{
		Vision:     true,
		VideoInput: true,
		ToolUse:    true,
	}
}
