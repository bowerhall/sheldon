package tools

import (
	"context"
	"fmt"
	"strings"

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
const SessionIDKey ctxKey = "sessionID"

func ChatIDFromContext(ctx context.Context) int64 {
	if id, ok := ctx.Value(ChatIDKey).(int64); ok {
		return id
	}
	return 0
}

func SessionIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(SessionIDKey).(string); ok {
		return id
	}
	return ""
}

// UserEntityName returns the entity name for the current user based on session
func UserEntityName(ctx context.Context) string {
	sessionID := SessionIDFromContext(ctx)
	if sessionID == "" {
		// fallback to chatID-based name
		return fmt.Sprintf("user_unknown_%d", ChatIDFromContext(ctx))
	}
	// sessionID format: "provider:chatID" -> entity name: "user_provider_chatID"
	parts := strings.SplitN(sessionID, ":", 2)
	if len(parts) == 2 {
		return fmt.Sprintf("user_%s_%s", parts[0], parts[1])
	}
	return sessionID
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
