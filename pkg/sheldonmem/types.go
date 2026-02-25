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
	// SalienceThreshold: facts with salience score below this are candidates for decay.
	// Score combines age, recency of access, and access count.
	// Range 0-1, lower = less important. Default 0.2
	SalienceThreshold float64
}

var DefaultDecayConfig = DecayConfig{
	MaxAge:            180 * 24 * time.Hour, // 6 months base age
	MaxAccessCount:    1,                    // legacy: facts accessed <= 1 time
	MaxConfidence:     0.5,                  // low confidence facts
	SalienceThreshold: 0.2,                  // salience score threshold
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
	Sensitive    bool
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
