package llm

import "context"

type Config struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string
}

type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
	MediaTypePDF   MediaType = "pdf"
)

type MediaContent struct {
	Type      MediaType
	Data      []byte
	MimeType  string
}

type Message struct {
	Role       string
	Content    string
	Media      []MediaContent
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

type Capabilities struct {
	Vision      bool
	VideoInput  bool
	PDFInput    bool
	ToolUse     bool
}

type LLM interface {
	Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error)
	ChatWithTools(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*ChatResponse, error)
	Capabilities() Capabilities
	Provider() string
	Model() string
}
