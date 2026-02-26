package sheldonmem

import (
	"database/sql"
	"time"
)

// SaveNote creates or updates a note
func (s *Store) SaveNote(key, content string) error {
	_, err := s.db.Exec(`
		INSERT INTO notes (key, content, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET
			content = excluded.content,
			updated_at = datetime('now')
	`, key, content)
	return err
}

// GetNote retrieves a note by key
func (s *Store) GetNote(key string) (*Note, error) {
	var note Note
	err := s.db.QueryRow(`
		SELECT key, content, updated_at
		FROM notes WHERE key = ?
	`, key).Scan(&note.Key, &note.Content, &note.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &note, nil
}

// DeleteNote removes a note by key
func (s *Store) DeleteNote(key string) error {
	_, err := s.db.Exec(`DELETE FROM notes WHERE key = ?`, key)
	return err
}

// ListNotes returns all note keys
func (s *Store) ListNotes() ([]string, error) {
	rows, err := s.db.Query(`SELECT key FROM notes ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// ListNotesWithAge returns note keys with their age for context display
func (s *Store) ListNotesWithAge() ([]NoteInfo, error) {
	rows, err := s.db.Query(`SELECT key, updated_at FROM notes ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []NoteInfo
	for rows.Next() {
		var info NoteInfo
		if err := rows.Scan(&info.Key, &info.UpdatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, info)
	}
	return notes, rows.Err()
}

// NoteInfo contains key and metadata for listing
type NoteInfo struct {
	Key       string
	UpdatedAt time.Time
}
