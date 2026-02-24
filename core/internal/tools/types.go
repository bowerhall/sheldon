package tools

import (
	"context"

	"github.com/bowerhall/sheldon/internal/llm"
)

type Handler func(ctx context.Context, args string) (string, error)

type NotifyFunc func(chatID int64, message string)

type Registry struct {
	tools    []llm.Tool
	handlers map[string]Handler
	notify   NotifyFunc
}

type ctxKey string

const ChatIDKey ctxKey = "chatID"
const ImagesKey ctxKey = "images"

func ChatIDFromContext(ctx context.Context) int64 {
	if id, ok := ctx.Value(ChatIDKey).(int64); ok {
		return id
	}
	return 0
}

func ImagesFromContext(ctx context.Context) []llm.ImageContent {
	if images, ok := ctx.Value(ImagesKey).([]llm.ImageContent); ok {
		return images
	}
	return nil
}
