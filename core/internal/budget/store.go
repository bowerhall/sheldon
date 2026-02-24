package budget

import (
	"database/sql"
	"time"
)

const schema = `
CREATE TABLE IF NOT EXISTS usage (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME NOT NULL,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	input_tokens INTEGER NOT NULL,
	output_tokens INTEGER NOT NULL,
	cost_usd REAL NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_timestamp ON usage(timestamp);
CREATE INDEX IF NOT EXISTS idx_usage_provider ON usage(provider);
CREATE INDEX IF NOT EXISTS idx_usage_model ON usage(model);
`

type Store struct {
	db       *sql.DB
	timezone *time.Location
}

func NewStore(db *sql.DB, timezone *time.Location) (*Store, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	tz := timezone
	if tz == nil {
		tz = time.UTC
	}

	return &Store{db: db, timezone: tz}, nil
}

type UsageRecord struct {
	Timestamp    time.Time
	Provider     string
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}

func (s *Store) Record(provider, model string, inputTokens, outputTokens int) error {
	cost := CalculateCost(model, inputTokens, outputTokens)

	_, err := s.db.Exec(
		`INSERT INTO usage (timestamp, provider, model, input_tokens, output_tokens, cost_usd) VALUES (?, ?, ?, ?, ?, ?)`,
		time.Now().In(s.timezone),
		provider,
		model,
		inputTokens,
		outputTokens,
		cost,
	)

	return err
}

type Summary struct {
	TotalRequests    int
	TotalInputTokens int
	TotalOutputTokens int
	TotalCostUSD     float64
}

func (s *Store) SummaryRange(from, to time.Time) (*Summary, error) {
	row := s.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cost_usd), 0)
		FROM usage
		WHERE timestamp >= ? AND timestamp < ?
	`, from, to)

	var sum Summary
	if err := row.Scan(&sum.TotalRequests, &sum.TotalInputTokens, &sum.TotalOutputTokens, &sum.TotalCostUSD); err != nil {
		return nil, err
	}

	return &sum, nil
}

func (s *Store) Today() (*Summary, error) {
	now := time.Now().In(s.timezone)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.timezone)
	end := start.Add(24 * time.Hour)

	return s.SummaryRange(start, end)
}

func (s *Store) ThisWeek() (*Summary, error) {
	now := time.Now().In(s.timezone)
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, s.timezone)
	end := start.Add(7 * 24 * time.Hour)

	return s.SummaryRange(start, end)
}

func (s *Store) ThisMonth() (*Summary, error) {
	now := time.Now().In(s.timezone)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.timezone)
	end := start.AddDate(0, 1, 0)

	return s.SummaryRange(start, end)
}

type ModelBreakdown struct {
	Model        string
	Requests     int
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}

func (s *Store) BreakdownByModel(from, to time.Time) ([]ModelBreakdown, error) {
	rows, err := s.db.Query(`
		SELECT
			model,
			COUNT(*),
			SUM(input_tokens),
			SUM(output_tokens),
			SUM(cost_usd)
		FROM usage
		WHERE timestamp >= ? AND timestamp < ?
		GROUP BY model
		ORDER BY SUM(cost_usd) DESC
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ModelBreakdown
	for rows.Next() {
		var b ModelBreakdown
		if err := rows.Scan(&b.Model, &b.Requests, &b.InputTokens, &b.OutputTokens, &b.CostUSD); err != nil {
			return nil, err
		}
		result = append(result, b)
	}

	return result, rows.Err()
}

type DailyBreakdown struct {
	Date         string
	Requests     int
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}

func (s *Store) BreakdownByDay(from, to time.Time) ([]DailyBreakdown, error) {
	rows, err := s.db.Query(`
		SELECT
			DATE(timestamp),
			COUNT(*),
			SUM(input_tokens),
			SUM(output_tokens),
			SUM(cost_usd)
		FROM usage
		WHERE timestamp >= ? AND timestamp < ?
		GROUP BY DATE(timestamp)
		ORDER BY DATE(timestamp) DESC
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DailyBreakdown
	for rows.Next() {
		var b DailyBreakdown
		if err := rows.Scan(&b.Date, &b.Requests, &b.InputTokens, &b.OutputTokens, &b.CostUSD); err != nil {
			return nil, err
		}
		result = append(result, b)
	}

	return result, rows.Err()
}
