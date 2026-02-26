package sheldonmem

import (
	"database/sql"
	"time"
)

// Note tiers
const (
	TierWorking = "working"
	TierArchive = "archive"
)

// SaveNote creates or updates a working note
func (s *Store) SaveNote(key, content string) error {
	_, err := s.db.Exec(`
		INSERT INTO notes (key, content, tier, updated_at)
		VALUES (?, ?, 'working', datetime('now'))
		ON CONFLICT(key) DO UPDATE SET
			content = excluded.content,
			tier = 'working',
			updated_at = datetime('now')
	`, key, content)
	return err
}

// GetNote retrieves a note by key (searches both tiers)
func (s *Store) GetNote(key string) (*Note, error) {
	var note Note
	err := s.db.QueryRow(`
		SELECT key, content, tier, updated_at
		FROM notes WHERE key = ?
	`, key).Scan(&note.Key, &note.Content, &note.Tier, &note.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &note, nil
}

// DeleteNote removes a note by key (any tier)
func (s *Store) DeleteNote(key string) error {
	_, err := s.db.Exec(`DELETE FROM notes WHERE key = ?`, key)
	return err
}

// ListNotes returns working note keys only
func (s *Store) ListNotes() ([]string, error) {
	rows, err := s.db.Query(`SELECT key FROM notes WHERE tier = 'working' ORDER BY updated_at DESC`)
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

// ListNotesWithAge returns working note keys with age for system prompt
func (s *Store) ListNotesWithAge() ([]NoteInfo, error) {
	rows, err := s.db.Query(`SELECT key, updated_at FROM notes WHERE tier = 'working' ORDER BY updated_at DESC`)
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

// ArchiveNote moves a working note to archive tier with a new key
func (s *Store) ArchiveNote(oldKey, newKey string) error {
	// Get the current note
	note, err := s.GetNote(oldKey)
	if err != nil {
		return err
	}
	if note == nil {
		return sql.ErrNoRows
	}

	// Insert/update with new key in archive tier
	_, err = s.db.Exec(`
		INSERT INTO notes (key, content, tier, updated_at)
		VALUES (?, ?, 'archive', datetime('now'))
		ON CONFLICT(key) DO UPDATE SET
			content = excluded.content,
			tier = 'archive',
			updated_at = datetime('now')
	`, newKey, note.Content)
	if err != nil {
		return err
	}

	// Delete the old working note
	return s.DeleteNote(oldKey)
}

// RestoreNote moves an archived note back to working tier
func (s *Store) RestoreNote(key string) error {
	_, err := s.db.Exec(`
		UPDATE notes SET tier = 'working', updated_at = datetime('now')
		WHERE key = ? AND tier = 'archive'
	`, key)
	return err
}

// ListArchivedNotes returns archived note keys matching an optional pattern
func (s *Store) ListArchivedNotes(pattern string) ([]NoteInfo, error) {
	var rows *sql.Rows
	var err error

	if pattern == "" {
		rows, err = s.db.Query(`
			SELECT key, updated_at FROM notes
			WHERE tier = 'archive'
			ORDER BY updated_at DESC
		`)
	} else {
		rows, err = s.db.Query(`
			SELECT key, updated_at FROM notes
			WHERE tier = 'archive' AND key LIKE ?
			ORDER BY updated_at DESC
		`, "%"+pattern+"%")
	}
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
