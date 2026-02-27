package session

import (
	"sync"

	"github.com/bowerhall/sheldon/internal/llm"
)

// QueuedMessage represents a message waiting to be processed
type QueuedMessage struct {
	Content string
	Media   []llm.MediaContent
	Trusted bool
}

type Session struct {
	mu         sync.Mutex
	messages   []llm.Message
	processing sync.Mutex
	queue      []QueuedMessage
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}
