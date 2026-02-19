package tools

import (
	"context"

	"github.com/kadet/kora/internal/llm"
)

func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

func (r *Registry) Register(tool llm.Tool, handler Handler) {
	r.tools = append(r.tools, tool)
	r.handlers[tool.Name] = handler
}

func (r *Registry) Tools() []llm.Tool {
	return r.tools
}

func (r *Registry) Execute(ctx context.Context, name, args string) (string, error) {
	handler, ok := r.handlers[name]
	if !ok {
		return "", nil
	}
	return handler(ctx, args)
}
