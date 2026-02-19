package session

import "github.com/kadet/kora/internal/llm"

func (s *Session) AddMessage(role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, llm.Message{Role: role, Content: content})
}

func (s *Session) Messages() []llm.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	copied := make([]llm.Message, len(s.messages))
	copy(copied, s.messages)

	return copied
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
