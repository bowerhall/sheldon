package llm

import "context"

type Config struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string
}

type ImageContent struct {
	Data      []byte
	MediaType string
}

type Message struct {
	Role       string
	Content    string
	Images     []ImageContent
	ToolCalls  []ToolCall
	ToolCallID string
}

type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type ChatResponse struct {
	Content    string
	ToolCalls  []ToolCall
	StopReason string
	Usage      *Usage
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type LLM interface {
	Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error)
	ChatWithTools(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*ChatResponse, error)
}
