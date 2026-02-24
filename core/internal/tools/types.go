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
const MediaKey ctxKey = "media"
const SafeModeKey ctxKey = "safeMode"

func ChatIDFromContext(ctx context.Context) int64 {
	if id, ok := ctx.Value(ChatIDKey).(int64); ok {
		return id
	}
	return 0
}

func MediaFromContext(ctx context.Context) []llm.MediaContent {
	if media, ok := ctx.Value(MediaKey).([]llm.MediaContent); ok {
		return media
	}
	return nil
}

func SafeModeFromContext(ctx context.Context) bool {
	if safe, ok := ctx.Value(SafeModeKey).(bool); ok {
		return safe
	}
	return false
}
