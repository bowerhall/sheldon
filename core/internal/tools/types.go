package tools

import (
	"context"

	"github.com/kadet/kora/internal/llm"
)

type Handler func(ctx context.Context, args string) (string, error)

type Registry struct {
	tools    []llm.Tool
	handlers map[string]Handler
}
