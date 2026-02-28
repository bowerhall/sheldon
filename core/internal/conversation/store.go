package conversation

import (
	"database/sql"
	"time"
)

const defaultMaxMessages = 12

type Message struct {
	Role      string
	Content   string
	CreatedAt time.Time
}

type Store struct {
	db          *sql.DB
	maxMessages int
}

const schema = `
CREATE TABLE IF NOT EXISTS recent_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_recent_messages_session ON recent_messages(session_id, created_at DESC);
`

// migration to convert old chat_id to session_id
const migrationSQL = `
-- Check if old table exists with chat_id column
-- SQLite doesn't support IF EXISTS for columns, so we handle this in Go
`

// NewStore creates a conversation buffer using the provided database connection
func NewStore(db *sql.DB, maxMessages int) (*Store, error) {
	if maxMessages <= 0 {
		maxMessages = defaultMaxMessages
	}
	s := &Store{db: db, maxMessages: maxMessages}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	// Check if old schema exists (chat_id column)
	var hasOldSchema bool
	err := s.db.QueryRow(`
		SELECT COUNT(*) > 0 FROM pragma_table_info('recent_messages')
		WHERE name = 'chat_id'
	`).Scan(&hasOldSchema)

	if err == nil && hasOldSchema {
		// Migrate old data: convert chat_id to session_id with telegram prefix
		// (assuming old data was telegram-only)
		_, _ = s.db.Exec(`
			ALTER TABLE recent_messages RENAME TO recent_messages_old;
		`)
		_, err = s.db.Exec(schema)
		if err != nil {
			return err
		}
		_, _ = s.db.Exec(`
			INSERT INTO recent_messages (session_id, role, content, created_at)
			SELECT 'telegram:' || chat_id, role, content, created_at
			FROM recent_messages_old;
		`)
		_, _ = s.db.Exec(`DROP TABLE recent_messages_old;`)
		return nil
	}

	// Fresh install
	_, err = s.db.Exec(schema)
	return err
}

// AddResult contains info about the add operation
type AddResult struct {
	Overflow []Message // Messages that were evicted from the buffer
}

func (s *Store) Add(sessionID string, role, content string) (*AddResult, error) {
	result := &AddResult{}

	// First, check if we'll overflow and capture those messages
	rows, err := s.db.Query(`
		SELECT role, content, created_at
		FROM recent_messages
		WHERE session_id = ?
		ORDER BY created_at ASC
		LIMIT ?`,
		sessionID, s.maxMessages)
	if err != nil {
		return nil, err
	}

	var existing []Message
	for rows.Next() {
		var m Message
		var createdAt string
		if err := rows.Scan(&m.Role, &m.Content, &createdAt); err != nil {
			rows.Close()
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		existing = append(existing, m)
	}
	rows.Close()

	// If buffer is full, the oldest messages will be evicted
	// After adding 2 new messages (user + assistant), we need to evict 2
	if len(existing) >= s.maxMessages {
		// Return the messages that will be evicted (oldest ones)
		evictCount := len(existing) - s.maxMessages + 2 // +2 for incoming user+assistant
		if evictCount > 0 && evictCount <= len(existing) {
			result.Overflow = existing[:evictCount]
		}
	}

	// Insert new message
	_, err = s.db.Exec(
		`INSERT INTO recent_messages (session_id, role, content) VALUES (?, ?, ?)`,
		sessionID, role, content,
	)
	if err != nil {
		return nil, err
	}

	// trim to max messages (FIFO)
	_, err = s.db.Exec(`
		DELETE FROM recent_messages
		WHERE session_id = ? AND id NOT IN (
			SELECT id FROM recent_messages
			WHERE session_id = ?
			ORDER BY created_at DESC
			LIMIT ?
		)`, sessionID, sessionID, s.maxMessages)

	return result, err
}

func (s *Store) GetRecent(sessionID string) ([]Message, error) {
	rows, err := s.db.Query(`
		SELECT role, content, created_at
		FROM recent_messages
		WHERE session_id = ?
		ORDER BY created_at ASC
		LIMIT ?`, sessionID, s.maxMessages)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var createdAt string
		if err := rows.Scan(&m.Role, &m.Content, &createdAt); err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		messages = append(messages, m)
	}

	return messages, nil
}

func (s *Store) Clear(sessionID string) error {
	_, err := s.db.Exec(`DELETE FROM recent_messages WHERE session_id = ?`, sessionID)
	return err
}
