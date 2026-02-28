package conversation

import (
	"database/sql"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func TestStoreAddAndGetRecent(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	store, err := NewStore(db, 10)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	sessionID := "telegram:123"

	err = store.Add(sessionID, "user", "hello")
	if err != nil {
		t.Fatalf("failed to add message: %v", err)
	}

	err = store.Add(sessionID, "assistant", "hi there")
	if err != nil {
		t.Fatalf("failed to add message: %v", err)
	}

	messages, err := store.GetRecent(sessionID)
	if err != nil {
		t.Fatalf("failed to get recent: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// verify both messages exist (order may vary due to same-second timestamps)
	hasUser := false
	hasAssistant := false
	for _, m := range messages {
		if m.Role == "user" && m.Content == "hello" {
			hasUser = true
		}
		if m.Role == "assistant" && m.Content == "hi there" {
			hasAssistant = true
		}
	}

	if !hasUser {
		t.Error("missing user message")
	}
	if !hasAssistant {
		t.Error("missing assistant message")
	}
}

func TestStoreMaxMessages(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	maxMessages := 5
	store, err := NewStore(db, maxMessages)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	sessionID := "telegram:123"

	// add more than max
	for i := 0; i < 10; i++ {
		err = store.Add(sessionID, "user", "message")
		if err != nil {
			t.Fatalf("failed to add message: %v", err)
		}
	}

	messages, err := store.GetRecent(sessionID)
	if err != nil {
		t.Fatalf("failed to get recent: %v", err)
	}

	if len(messages) != maxMessages {
		t.Errorf("expected %d messages (max), got %d", maxMessages, len(messages))
	}
}

func TestStoreSessionIsolation(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	store, err := NewStore(db, 10)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session1 := "telegram:111"
	session2 := "discord:222"

	store.Add(session1, "user", "session1 message")
	store.Add(session2, "user", "session2 message")

	messages1, _ := store.GetRecent(session1)
	messages2, _ := store.GetRecent(session2)

	if len(messages1) != 1 {
		t.Errorf("expected 1 message for session1, got %d", len(messages1))
	}

	if len(messages2) != 1 {
		t.Errorf("expected 1 message for session2, got %d", len(messages2))
	}

	if messages1[0].Content != "session1 message" {
		t.Errorf("session1 content mismatch: %s", messages1[0].Content)
	}

	if messages2[0].Content != "session2 message" {
		t.Errorf("session2 content mismatch: %s", messages2[0].Content)
	}
}

func TestStoreClear(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	store, err := NewStore(db, 10)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	sessionID := "telegram:123"

	store.Add(sessionID, "user", "hello")
	store.Add(sessionID, "assistant", "hi")

	err = store.Clear(sessionID)
	if err != nil {
		t.Fatalf("failed to clear: %v", err)
	}

	messages, _ := store.GetRecent(sessionID)
	if len(messages) != 0 {
		t.Errorf("expected 0 messages after clear, got %d", len(messages))
	}
}

func TestStoreDefaultMaxMessages(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// pass 0 to use default
	store, err := NewStore(db, 0)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	sessionID := "test"

	// add more than default (12)
	for i := 0; i < 20; i++ {
		store.Add(sessionID, "user", "msg")
	}

	messages, _ := store.GetRecent(sessionID)
	if len(messages) != defaultMaxMessages {
		t.Errorf("expected default %d messages, got %d", defaultMaxMessages, len(messages))
	}
}
