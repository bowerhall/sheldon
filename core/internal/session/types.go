package session

import (
	"sync"

	"github.com/kadet/kora/internal/llm"
)

type Session struct {
	mu       sync.Mutex
	messages []llm.Message
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}
