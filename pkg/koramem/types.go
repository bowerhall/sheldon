package koramem

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
