package session

import (
	"sync"
	"testing"

	"github.com/bowerhall/sheldon/internal/llm"
)

func TestSessionAddAndGetMessages(t *testing.T) {
	s := &Session{}

	s.AddMessage("user", "hello", nil, "")
	s.AddMessage("assistant", "hi there", nil, "")

	msgs := s.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Errorf("first message mismatch: %+v", msgs[0])
	}

	if msgs[1].Role != "assistant" || msgs[1].Content != "hi there" {
		t.Errorf("second message mismatch: %+v", msgs[1])
	}
}

func TestSessionAddMessageWithToolCalls(t *testing.T) {
	s := &Session{}

	toolCalls := []llm.ToolCall{
		{ID: "call_1", Name: "recall_memory", Arguments: `{"query":"test"}`},
	}

	s.AddMessage("assistant", "Let me check", toolCalls, "")

	msgs := s.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	if len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msgs[0].ToolCalls))
	}

	if msgs[0].ToolCalls[0].Name != "recall_memory" {
		t.Errorf("expected tool 'recall_memory', got '%s'", msgs[0].ToolCalls[0].Name)
	}
}

func TestSessionAddMessageWithMedia(t *testing.T) {
	s := &Session{}

	media := []llm.MediaContent{
		{Type: llm.MediaTypeImage, Data: []byte("fake image")},
	}

	s.AddMessageWithMedia("user", "look at this", media, nil, "")

	msgs := s.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	if len(msgs[0].Media) != 1 {
		t.Fatalf("expected 1 media, got %d", len(msgs[0].Media))
	}

	if msgs[0].Media[0].Type != llm.MediaTypeImage {
		t.Errorf("expected image type, got %s", msgs[0].Media[0].Type)
	}
}

func TestSessionMessagesIsCopy(t *testing.T) {
	s := &Session{}
	s.AddMessage("user", "hello", nil, "")

	msgs := s.Messages()
	msgs[0].Content = "modified"

	// original should be unchanged
	original := s.Messages()
	if original[0].Content != "hello" {
		t.Error("Messages() should return a copy, not the original slice")
	}
}

func TestSessionTryAcquireAndRelease(t *testing.T) {
	s := &Session{}

	// first acquire should succeed
	if !s.TryAcquire() {
		t.Error("first TryAcquire should succeed")
	}

	// second acquire should fail (already processing)
	if s.TryAcquire() {
		t.Error("second TryAcquire should fail")
	}

	// release and try again
	s.Release()

	if !s.TryAcquire() {
		t.Error("TryAcquire after Release should succeed")
	}
	s.Release()
}

func TestSessionQueue(t *testing.T) {
	s := &Session{}

	// queue should be empty initially
	if msg := s.Dequeue(); msg != nil {
		t.Error("expected nil from empty queue")
	}

	if s.QueueLen() != 0 {
		t.Errorf("expected queue length 0, got %d", s.QueueLen())
	}

	// add to queue
	s.Queue("message 1", nil, true)
	s.Queue("message 2", nil, false)

	if s.QueueLen() != 2 {
		t.Errorf("expected queue length 2, got %d", s.QueueLen())
	}

	// dequeue FIFO
	msg1 := s.Dequeue()
	if msg1 == nil || msg1.Content != "message 1" || !msg1.Trusted {
		t.Errorf("first dequeue mismatch: %+v", msg1)
	}

	msg2 := s.Dequeue()
	if msg2 == nil || msg2.Content != "message 2" || msg2.Trusted {
		t.Errorf("second dequeue mismatch: %+v", msg2)
	}

	if s.QueueLen() != 0 {
		t.Errorf("expected queue length 0 after dequeue, got %d", s.QueueLen())
	}
}

func TestSessionConcurrentAccess(t *testing.T) {
	s := &Session{}
	var wg sync.WaitGroup

	// concurrent message adds
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.AddMessage("user", "message", nil, "")
		}(i)
	}

	wg.Wait()

	msgs := s.Messages()
	if len(msgs) != 100 {
		t.Errorf("expected 100 messages, got %d", len(msgs))
	}
}

func TestStoreGetCreatesSession(t *testing.T) {
	store := NewStore()

	sess1 := store.Get("telegram:123")
	if sess1 == nil {
		t.Fatal("Get should create new session")
	}

	// same ID should return same session
	sess2 := store.Get("telegram:123")
	if sess1 != sess2 {
		t.Error("Get should return same session for same ID")
	}
}

func TestStoreGetDifferentSessions(t *testing.T) {
	store := NewStore()

	sess1 := store.Get("telegram:111")
	sess2 := store.Get("discord:222")

	if sess1 == sess2 {
		t.Error("different IDs should get different sessions")
	}

	sess1.AddMessage("user", "telegram message", nil, "")
	sess2.AddMessage("user", "discord message", nil, "")

	if len(sess1.Messages()) != 1 || sess1.Messages()[0].Content != "telegram message" {
		t.Error("session 1 messages corrupted")
	}

	if len(sess2.Messages()) != 1 || sess2.Messages()[0].Content != "discord message" {
		t.Error("session 2 messages corrupted")
	}
}

func TestStoreConcurrentGet(t *testing.T) {
	store := NewStore()
	var wg sync.WaitGroup
	sessions := make(chan *Session, 100)

	// concurrent gets for same session
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sess := store.Get("shared:session")
			sessions <- sess
		}()
	}

	wg.Wait()
	close(sessions)

	// all should be the same session
	var first *Session
	for sess := range sessions {
		if first == nil {
			first = sess
		} else if sess != first {
			t.Error("concurrent Get returned different sessions for same ID")
		}
	}
}
