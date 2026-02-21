package sheldonmem

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// cronParser is configured for standard 5-field cron expressions
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// CreateCron creates a new scheduled reminder
func (s *Store) CreateCron(keyword, schedule string, chatID int64, expiresAt *time.Time) (*Cron, error) {
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

// GetDueCrons returns all crons that should fire now (next_run <= now and not expired)
func (s *Store) GetDueCrons() ([]Cron, error) {
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

// GetCronsByChat returns all active crons for a specific chat
func (s *Store) GetCronsByChat(chatID int64) ([]Cron, error) {
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

// UpdateCronNextRun updates the next run time for a cron
func (s *Store) UpdateCronNextRun(id int64, nextRun time.Time) error {
	_, err := s.db.Exec(`UPDATE crons SET next_run = ? WHERE id = ?`, nextRun, id)
	return err
}

// DeleteCron deletes a cron by ID
func (s *Store) DeleteCron(id int64) error {
	_, err := s.db.Exec(`DELETE FROM crons WHERE id = ?`, id)
	return err
}

// DeleteCronByKeyword deletes a cron by keyword and chat ID
func (s *Store) DeleteCronByKeyword(keyword string, chatID int64) error {
	_, err := s.db.Exec(`DELETE FROM crons WHERE keyword = ? AND chat_id = ?`, keyword, chatID)
	return err
}

// DeleteExpiredCrons removes all crons past their expiry date
func (s *Store) DeleteExpiredCrons() (int, error) {
	result, err := s.db.Exec(`DELETE FROM crons WHERE expires_at IS NOT NULL AND expires_at <= datetime('now')`)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// scanCrons is a helper to scan cron rows
func (s *Store) scanCrons(rows interface{ Next() bool; Scan(dest ...any) error }) ([]Cron, error) {
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
