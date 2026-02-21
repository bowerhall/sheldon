package cron

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// Cron represents a scheduled reminder
type Cron struct {
	ID        int64
	Keyword   string     // search term for memory recall
	Schedule  string     // cron expression "0 20 * * *"
	ChatID    int64      // where to send notification
	ExpiresAt *time.Time // auto-delete after this time (nil = never)
	NextRun   time.Time  // pre-computed next fire time
	CreatedAt time.Time
}

// Store manages cron persistence
type Store struct {
	db *sql.DB
}

// cronParser is configured for standard 5-field cron expressions
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

const schema = `
CREATE TABLE IF NOT EXISTS crons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    keyword TEXT NOT NULL,
    schedule TEXT NOT NULL,
    chat_id INTEGER NOT NULL,
    expires_at DATETIME,
    next_run DATETIME NOT NULL,
    created_at DATETIME DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_crons_next_run ON crons(next_run);
CREATE INDEX IF NOT EXISTS idx_crons_chat_id ON crons(chat_id);
`

// NewStore creates a cron store using the provided database connection
func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}

	if err := s.migrate(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(schema)
	return err
}

// Create creates a new scheduled reminder
func (s *Store) Create(keyword, schedule string, chatID int64, expiresAt *time.Time) (*Cron, error) {
	// validate cron expression
	sched, err := cronParser.Parse(schedule)
	if err != nil {
		return nil, fmt.Errorf("invalid cron schedule: %w", err)
	}

	nextRun := sched.Next(time.Now())

	result, err := s.db.Exec(`
		INSERT INTO crons (keyword, schedule, chat_id, expires_at, next_run)
		VALUES (?, ?, ?, ?, ?)`,
		keyword, schedule, chatID, expiresAt, nextRun)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &Cron{
		ID:        id,
		Keyword:   keyword,
		Schedule:  schedule,
		ChatID:    chatID,
		ExpiresAt: expiresAt,
		NextRun:   nextRun,
		CreatedAt: time.Now(),
	}, nil
}

// GetDue returns all crons that should fire now (next_run <= now and not expired)
func (s *Store) GetDue() ([]Cron, error) {
	rows, err := s.db.Query(`
		SELECT id, keyword, schedule, chat_id, expires_at, next_run, created_at
		FROM crons
		WHERE next_run <= datetime('now')
		AND (expires_at IS NULL OR expires_at > datetime('now'))`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanCrons(rows)
}

// GetByChat returns all active crons for a specific chat
func (s *Store) GetByChat(chatID int64) ([]Cron, error) {
	rows, err := s.db.Query(`
		SELECT id, keyword, schedule, chat_id, expires_at, next_run, created_at
		FROM crons
		WHERE chat_id = ?
		AND (expires_at IS NULL OR expires_at > datetime('now'))
		ORDER BY next_run ASC`,
		chatID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanCrons(rows)
}

// UpdateNextRun updates the next run time for a cron
func (s *Store) UpdateNextRun(id int64, nextRun time.Time) error {
	_, err := s.db.Exec(`UPDATE crons SET next_run = ? WHERE id = ?`, nextRun, id)
	return err
}

// Delete deletes a cron by ID
func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM crons WHERE id = ?`, id)
	return err
}

// DeleteByKeyword deletes a cron by keyword and chat ID
func (s *Store) DeleteByKeyword(keyword string, chatID int64) error {
	_, err := s.db.Exec(`DELETE FROM crons WHERE keyword = ? AND chat_id = ?`, keyword, chatID)
	return err
}

// DeleteExpired removes all crons past their expiry date
func (s *Store) DeleteExpired() (int, error) {
	result, err := s.db.Exec(`DELETE FROM crons WHERE expires_at IS NOT NULL AND expires_at <= datetime('now')`)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// scanCrons is a helper to scan cron rows
func (s *Store) scanCrons(rows *sql.Rows) ([]Cron, error) {
	var crons []Cron

	for rows.Next() {
		var c Cron
		var expiresAt, nextRun, createdAt *string

		err := rows.Scan(&c.ID, &c.Keyword, &c.Schedule, &c.ChatID, &expiresAt, &nextRun, &createdAt)
		if err != nil {
			return nil, err
		}

		if expiresAt != nil {
			t, _ := time.Parse("2006-01-02 15:04:05", *expiresAt)
			c.ExpiresAt = &t
		}

		if nextRun != nil {
			c.NextRun, _ = time.Parse("2006-01-02 15:04:05", *nextRun)
		}

		if createdAt != nil {
			c.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", *createdAt)
		}

		crons = append(crons, c)
	}

	return crons, nil
}

// ComputeNextRun calculates the next run time from a cron schedule
func ComputeNextRun(schedule string) (time.Time, error) {
	sched, err := cronParser.Parse(schedule)
	if err != nil {
		return time.Time{}, err
	}

	return sched.Next(time.Now()), nil
}
