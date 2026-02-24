package conversation

import (
	"database/sql"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

const maxMessages = 12

type Message struct {
	Role      string
	Content   string
	CreatedAt time.Time
}

type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS recent_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_recent_messages_chat ON recent_messages(chat_id, created_at DESC);
`

// NewStore creates a conversation buffer with its own SQLite database file
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) Add(chatID int64, role, content string) error {
	_, err := s.db.Exec(
		`INSERT INTO recent_messages (chat_id, role, content) VALUES (?, ?, ?)`,
		chatID, role, content,
	)
	if err != nil {
		return err
	}

	// trim to max messages (FIFO)
	_, err = s.db.Exec(`
		DELETE FROM recent_messages
		WHERE chat_id = ? AND id NOT IN (
			SELECT id FROM recent_messages
			WHERE chat_id = ?
			ORDER BY created_at DESC
			LIMIT ?
		)`, chatID, chatID, maxMessages)

	return err
}

func (s *Store) GetRecent(chatID int64) ([]Message, error) {
	rows, err := s.db.Query(`
		SELECT role, content, created_at
		FROM recent_messages
		WHERE chat_id = ?
		ORDER BY created_at ASC
		LIMIT ?`, chatID, maxMessages)
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

func (s *Store) Clear(chatID int64) error {
	_, err := s.db.Exec(`DELETE FROM recent_messages WHERE chat_id = ?`, chatID)
	return err
}
