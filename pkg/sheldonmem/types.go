package sheldonmem

import (
	"context"
	"database/sql"
	"time"
)

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type Store struct {
	db       *sql.DB
	embedder Embedder
}

type DecayConfig struct {
	MaxAge          time.Duration
	MaxAccessCount  int
	MaxConfidence   float64
	DomainOverrides map[int]time.Duration
}

var DefaultDecayConfig = DecayConfig{
	MaxAge:         180 * 24 * time.Hour, // 6 months
	MaxAccessCount: 1,
	MaxConfidence:  0.5,
}

type Domain struct {
	ID    int
	Name  string
	Slug  string
	Layer string
}

type Entity struct {
	ID         int64
	Name       string
	EntityType string
	DomainID   int
	Metadata   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Fact struct {
	ID           int64
	EntityID     *int64
	DomainID     int
	Field        string
	Value        string
	Confidence   float64
	AccessCount  int
	LastAccessed *time.Time
	Supersedes   *int64
	Active       bool
	CreatedAt    time.Time
}

type Edge struct {
	ID        int64
	SourceID  int64
	TargetID  int64
	Relation  string
	Strength  float64
	Metadata  string
	CreatedAt time.Time
}

// FactResult contains the stored fact and any contradiction info
type FactResult struct {
	Fact       *Fact
	Superseded *Fact // non-nil if this fact replaced an older one
}

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
