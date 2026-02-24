package session

import "github.com/bowerhall/sheldon/internal/llm"

func (s *Session) AddMessage(role, content string, toolCalls []llm.ToolCall, toolCallID string) {
	s.AddMessageWithMedia(role, content, nil, toolCalls, toolCallID)
}

func (s *Session) AddMessageWithMedia(role, content string, media []llm.MediaContent, toolCalls []llm.ToolCall, toolCallID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, llm.Message{
		Role:       role,
		Content:    content,
		Media:      media,
		ToolCalls:  toolCalls,
		ToolCallID: toolCallID,
	})
}

func (s *Session) Messages() []llm.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	copied := make([]llm.Message, len(s.messages))
	copy(copied, s.messages)

	return copied
}

// TryAcquire attempts to acquire the processing lock.
// Returns true if acquired, false if already processing.
func (s *Session) TryAcquire() bool {
	return s.processing.TryLock()
}

// Release releases the processing lock.
func (s *Session) Release() {
	s.processing.Unlock()
}

func NewStore() *Store {
	return &Store{sessions: make(map[string]*Session)}
}

func (s *Store) Get(sessionID string) *Session {
	s.mu.RLock()

	sess, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if ok {
		return sess
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok = s.sessions[sessionID]; ok {
		return sess
	}

	sess = &Session{}
	s.sessions[sessionID] = sess

	return sess
}
