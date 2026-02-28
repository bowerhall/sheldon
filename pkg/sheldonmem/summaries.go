package sheldonmem

import (
	"context"
	"database/sql"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/ncruces"
)

type ConversationChunk struct {
	ID        int64
	SessionID string
	Content   string
	CreatedAt time.Time
}

type DailySummary struct {
	ID          int64
	SessionID   string
	SummaryDate time.Time
	Summary     string
	CreatedAt   time.Time
}

// SaveChunk stores a raw conversation chunk (no LLM call, cheap)
func (s *Store) SaveChunk(sessionID, content string) error {
	_, err := s.db.Exec(
		`INSERT INTO conversation_chunks (session_id, content) VALUES (?, ?)`,
		sessionID, content,
	)
	return err
}

// GetChunksForDate retrieves all chunks for a session on a specific date
func (s *Store) GetChunksForDate(sessionID string, date time.Time) ([]ConversationChunk, error) {
	dateStr := date.Format("2006-01-02")
	rows, err := s.db.Query(
		`SELECT id, session_id, content, created_at
		 FROM conversation_chunks
		 WHERE session_id = ? AND date(created_at) = ?
		 ORDER BY created_at ASC`,
		sessionID, dateStr,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []ConversationChunk
	for rows.Next() {
		var c ConversationChunk
		if err := rows.Scan(&c.ID, &c.SessionID, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, nil
}

// GetPendingChunkDates returns dates that have chunks but no summary yet
func (s *Store) GetPendingChunkDates(sessionID string) ([]time.Time, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT date(created_at) as chunk_date
		 FROM conversation_chunks
		 WHERE session_id = ?
		   AND date(created_at) < date('now')
		   AND date(created_at) NOT IN (
		       SELECT summary_date FROM daily_summaries WHERE session_id = ?
		   )
		 ORDER BY chunk_date ASC`,
		sessionID, sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dates []time.Time
	for rows.Next() {
		var dateStr string
		if err := rows.Scan(&dateStr); err != nil {
			return nil, err
		}
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		dates = append(dates, date)
	}
	return dates, nil
}

// SaveDailySummary stores a daily summary and embeds it for semantic search
func (s *Store) SaveDailySummary(ctx context.Context, sessionID string, date time.Time, summary string) error {
	dateStr := date.Format("2006-01-02")

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert or replace summary
	result, err := tx.ExecContext(ctx,
		`INSERT INTO daily_summaries (session_id, summary_date, summary)
		 VALUES (?, ?, ?)
		 ON CONFLICT(session_id, summary_date) DO UPDATE SET summary = excluded.summary`,
		sessionID, dateStr, summary,
	)
	if err != nil {
		return err
	}

	summaryID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	// Generate and store embedding if embedder is available
	if s.embedder != nil {
		embedding, err := s.embedder.Embed(ctx, summary)
		if err == nil && len(embedding) > 0 {
			blob, err := sqlite_vec.SerializeFloat32(embedding)
			if err == nil {
				// Delete existing vector if any
				_, _ = tx.ExecContext(ctx, `DELETE FROM vec_summaries WHERE summary_id = ?`, summaryID)

				// Insert new vector
				_, err = tx.ExecContext(ctx,
					`INSERT INTO vec_summaries (summary_id, embedding) VALUES (?, ?)`,
					summaryID, blob,
				)
			}
			if err != nil {
				// Log but don't fail - summary is still useful without embedding
				return tx.Commit()
			}
		}
	}

	return tx.Commit()
}

// GetDailySummary retrieves a summary for a specific date
func (s *Store) GetDailySummary(sessionID string, date time.Time) (*DailySummary, error) {
	dateStr := date.Format("2006-01-02")

	var ds DailySummary
	err := s.db.QueryRow(
		`SELECT id, session_id, summary_date, summary, created_at
		 FROM daily_summaries
		 WHERE session_id = ? AND summary_date = ?`,
		sessionID, dateStr,
	).Scan(&ds.ID, &ds.SessionID, &ds.SummaryDate, &ds.Summary, &ds.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ds, nil
}

// SearchSummaries finds relevant summaries using semantic search
func (s *Store) SearchSummaries(ctx context.Context, sessionID string, query string, limit int) ([]DailySummary, error) {
	if s.embedder == nil {
		// Fall back to recent summaries if no embedder
		return s.GetRecentSummaries(sessionID, limit)
	}

	if limit <= 0 {
		limit = 5
	}

	embedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		// Fall back to recent summaries on embedding error
		return s.GetRecentSummaries(sessionID, limit)
	}

	blob, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return s.GetRecentSummaries(sessionID, limit)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT ds.id, ds.session_id, ds.summary_date, ds.summary, ds.created_at, v.distance
		 FROM vec_summaries v
		 JOIN daily_summaries ds ON ds.id = v.summary_id
		 WHERE ds.session_id = ?
		   AND v.embedding MATCH ?
		 ORDER BY v.distance ASC
		 LIMIT ?`,
		sessionID, blob, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []DailySummary
	for rows.Next() {
		var ds DailySummary
		var distance float64
		if err := rows.Scan(&ds.ID, &ds.SessionID, &ds.SummaryDate, &ds.Summary, &ds.CreatedAt, &distance); err != nil {
			return nil, err
		}
		summaries = append(summaries, ds)
	}
	return summaries, nil
}

// GetRecentSummaries retrieves the N most recent summaries for a session
func (s *Store) GetRecentSummaries(sessionID string, limit int) ([]DailySummary, error) {
	if limit <= 0 {
		limit = 7
	}

	rows, err := s.db.Query(
		`SELECT id, session_id, summary_date, summary, created_at
		 FROM daily_summaries
		 WHERE session_id = ?
		 ORDER BY summary_date DESC
		 LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []DailySummary
	for rows.Next() {
		var ds DailySummary
		if err := rows.Scan(&ds.ID, &ds.SessionID, &ds.SummaryDate, &ds.Summary, &ds.CreatedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, ds)
	}
	return summaries, nil
}

// DeleteOldChunks removes chunks older than the specified duration
// Call this after summaries are generated to free up space
func (s *Store) DeleteOldChunks(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.Exec(
		`DELETE FROM conversation_chunks WHERE created_at < ?`,
		cutoff,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
