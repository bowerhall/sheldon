package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/bowerhall/sheldon/internal/llm"
)

func TestRegistryRegisterAndExecute(t *testing.T) {
	r := NewRegistry()

	tool := llm.Tool{
		Name:        "test_tool",
		Description: "A test tool",
	}

	called := false
	r.Register(tool, func(ctx context.Context, args string) (string, error) {
		called = true
		return "result:" + args, nil
	})

	result, err := r.Execute(context.Background(), "test_tool", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Error("handler was not called")
	}

	if result != "result:hello" {
		t.Errorf("expected 'result:hello', got '%s'", result)
	}
}

func TestRegistryExecuteUnknownTool(t *testing.T) {
	r := NewRegistry()

	result, err := r.Execute(context.Background(), "nonexistent", "args")
	if err != nil {
		t.Errorf("expected no error for unknown tool, got: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result for unknown tool, got: %s", result)
	}
}

func TestRegistryExecuteWithError(t *testing.T) {
	r := NewRegistry()

	expectedErr := errors.New("tool failed")
	r.Register(llm.Tool{Name: "failing_tool"}, func(ctx context.Context, args string) (string, error) {
		return "", expectedErr
	})

	_, err := r.Execute(context.Background(), "failing_tool", "")
	if err != expectedErr {
		t.Errorf("expected error '%v', got '%v'", expectedErr, err)
	}
}

func TestRegistryTools(t *testing.T) {
	r := NewRegistry()

	r.Register(llm.Tool{Name: "tool1", Description: "First"}, nil)
	r.Register(llm.Tool{Name: "tool2", Description: "Second"}, nil)
	r.Register(llm.Tool{Name: "tool3", Description: "Third"}, nil)

	tools := r.Tools()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}

	for _, name := range []string{"tool1", "tool2", "tool3"} {
		if !names[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestRegistryNotify(t *testing.T) {
	r := NewRegistry()

	var notifiedChatID int64
	var notifiedMessage string

	r.SetNotify(func(chatID int64, message string) {
		notifiedChatID = chatID
		notifiedMessage = message
	})

	ctx := context.WithValue(context.Background(), ChatIDKey, int64(12345))
	r.Notify(ctx, "hello user")

	if notifiedChatID != 12345 {
		t.Errorf("expected chatID 12345, got %d", notifiedChatID)
	}
	if notifiedMessage != "hello user" {
		t.Errorf("expected message 'hello user', got '%s'", notifiedMessage)
	}
}

func TestRegistryNotifyNoChatID(t *testing.T) {
	r := NewRegistry()

	called := false
	r.SetNotify(func(chatID int64, message string) {
		called = true
	})

	// no chatID in context
	r.Notify(context.Background(), "hello")

	if called {
		t.Error("notify should not be called when no chatID in context")
	}
}

func TestRegistryNotifyNoHandler(t *testing.T) {
	r := NewRegistry()
	// no notify handler set - should not panic
	ctx := context.WithValue(context.Background(), ChatIDKey, int64(123))
	r.Notify(ctx, "hello") // should be no-op
}

func TestChatIDFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), ChatIDKey, int64(999))
	if id := ChatIDFromContext(ctx); id != 999 {
		t.Errorf("expected 999, got %d", id)
	}

	// no value
	if id := ChatIDFromContext(context.Background()); id != 0 {
		t.Errorf("expected 0 for missing chatID, got %d", id)
	}
}

func TestSessionIDFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), SessionIDKey, "telegram:123")
	if id := SessionIDFromContext(ctx); id != "telegram:123" {
		t.Errorf("expected 'telegram:123', got '%s'", id)
	}

	// no value
	if id := SessionIDFromContext(context.Background()); id != "" {
		t.Errorf("expected empty for missing sessionID, got '%s'", id)
	}
}

func TestUserEntityName(t *testing.T) {
	tests := []struct {
		sessionID string
		want      string
	}{
		{"telegram:123456", "user_telegram_123456"},
		{"discord:789", "user_discord_789"},
		{"", "user_unknown_0"},
	}

	for _, tt := range tests {
		ctx := context.Background()
		if tt.sessionID != "" {
			ctx = context.WithValue(ctx, SessionIDKey, tt.sessionID)
		}

		got := UserEntityName(ctx)
		if got != tt.want {
			t.Errorf("UserEntityName(%s) = %s, want %s", tt.sessionID, got, tt.want)
		}
	}
}

func TestSafeModeFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), SafeModeKey, true)
	if !SafeModeFromContext(ctx) {
		t.Error("expected safe mode true")
	}

	if SafeModeFromContext(context.Background()) {
		t.Error("expected safe mode false when not set")
	}
}
