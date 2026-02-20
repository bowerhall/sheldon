package tools

import (
	"context"

	"github.com/bowerhall/sheldon/internal/llm"
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

func (r *Registry) SetNotify(fn NotifyFunc) {
	r.notify = fn
}

func (r *Registry) Notify(ctx context.Context, message string) {
	if r.notify == nil {
		return
	}
	chatID := ChatIDFromContext(ctx)
	if chatID != 0 {
		r.notify(chatID, message)
	}
}
