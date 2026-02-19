package llm

import "context"

type Config struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string
}

type Message struct {
	Role    string
	Content string
}

type LLM interface {
	Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error)
}
