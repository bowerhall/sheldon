package budget

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func TestTrackerAdd(t *testing.T) {
	tracker := NewTracker(Config{DailyLimit: 1000, WarnAt: 0.8}, nil, nil)

	ok := tracker.Add(500)
	if !ok {
		t.Error("expected Add to return true when under limit")
	}

	used, limit := tracker.Usage()
	if used != 500 {
		t.Errorf("expected 500 used, got %d", used)
	}
	if limit != 1000 {
		t.Errorf("expected 1000 limit, got %d", limit)
	}
}

func TestTrackerExceedsLimit(t *testing.T) {
	exceededCalled := false
	tracker := NewTracker(Config{DailyLimit: 1000, WarnAt: 0.8}, nil, func(used, limit int) {
		exceededCalled = true
	})

	tracker.Add(500)
	ok := tracker.Add(600) // total 1100, exceeds 1000
	if ok {
		t.Error("expected Add to return false when exceeding limit")
	}
	if !exceededCalled {
		t.Error("expected onExceeded callback to be called")
	}
}

func TestTrackerWarning(t *testing.T) {
	warnCalled := false
	tracker := NewTracker(Config{DailyLimit: 1000, WarnAt: 0.8}, func(used, limit int) {
		warnCalled = true
	}, nil)

	tracker.Add(700) // 70%, no warning yet
	if warnCalled {
		t.Error("expected no warning at 70%")
	}

	tracker.Add(100) // 80%, should trigger warning
	if !warnCalled {
		t.Error("expected warning at 80%")
	}
}

func TestTrackerWarnOnlyOnce(t *testing.T) {
	warnCount := 0
	tracker := NewTracker(Config{DailyLimit: 1000, WarnAt: 0.8}, func(used, limit int) {
		warnCount++
	}, nil)

	tracker.Add(800) // triggers warning
	tracker.Add(50)  // should not trigger again
	tracker.Add(50)  // should not trigger again

	if warnCount != 1 {
		t.Errorf("expected warning to be called once, got %d", warnCount)
	}
}

func TestTrackerRecord(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	store, err := NewStore(db, time.UTC)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	tracker := NewTracker(Config{DailyLimit: 100000, WarnAt: 0.8}, nil, nil)
	tracker.SetStore(store)

	ok := tracker.Record("claude", "claude-sonnet-4-20250514", 1000, 100)
	if !ok {
		t.Error("expected Record to return true")
	}

	used, _ := tracker.Usage()
	if used != 1100 {
		t.Errorf("expected 1100 used, got %d", used)
	}

	// verify stored in db
	tokens, err := store.TodayTokens()
	if err != nil {
		t.Fatalf("failed to get today tokens: %v", err)
	}
	if tokens != 1100 {
		t.Errorf("expected 1100 tokens in store, got %d", tokens)
	}
}

func TestStoreRecord(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	store, err := NewStore(db, time.UTC)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	err = store.Record("claude", "claude-sonnet-4-20250514", 1000, 100)
	if err != nil {
		t.Fatalf("failed to record: %v", err)
	}

	summary, err := store.Today()
	if err != nil {
		t.Fatalf("failed to get today summary: %v", err)
	}

	if summary.TotalRequests != 1 {
		t.Errorf("expected 1 request, got %d", summary.TotalRequests)
	}
	if summary.TotalInputTokens != 1000 {
		t.Errorf("expected 1000 input tokens, got %d", summary.TotalInputTokens)
	}
	if summary.TotalOutputTokens != 100 {
		t.Errorf("expected 100 output tokens, got %d", summary.TotalOutputTokens)
	}
}

func TestStoreSummaryRange(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	store, err := NewStore(db, time.UTC)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// record multiple
	store.Record("claude", "claude-sonnet-4-20250514", 1000, 100)
	store.Record("openai", "gpt-4o", 500, 50)
	store.Record("kimi", "kimi-k2-0711-preview", 2000, 200)

	summary, err := store.Today()
	if err != nil {
		t.Fatalf("failed to get today summary: %v", err)
	}

	if summary.TotalRequests != 3 {
		t.Errorf("expected 3 requests, got %d", summary.TotalRequests)
	}
	if summary.TotalInputTokens != 3500 {
		t.Errorf("expected 3500 input tokens, got %d", summary.TotalInputTokens)
	}
	if summary.TotalOutputTokens != 350 {
		t.Errorf("expected 350 output tokens, got %d", summary.TotalOutputTokens)
	}
}

func TestPricingKnownModels(t *testing.T) {
	tests := []struct {
		model  string
		input  int
		output int
		want   float64
	}{
		{"claude-sonnet-4-20250514", 1000000, 0, 3.00},
		{"claude-sonnet-4-20250514", 0, 1000000, 15.00},
		{"gpt-4o", 1000000, 0, 2.50},
		{"gpt-4o", 0, 1000000, 10.00},
	}

	for _, tt := range tests {
		cost := CalculateCost(tt.model, tt.input, tt.output)
		if cost != tt.want {
			t.Errorf("CalculateCost(%s, %d, %d) = %f, want %f", tt.model, tt.input, tt.output, cost, tt.want)
		}
	}
}

func TestPricingOllamaFree(t *testing.T) {
	cost := CalculateCost("ollama/qwen2.5:3b", 1000000, 1000000)
	if cost != 0 {
		t.Errorf("expected ollama models to be free, got %f", cost)
	}

	cost = CalculateCost("qwen2.5:3b", 1000000, 1000000)
	if cost != 0 {
		t.Errorf("expected local models with : to be free, got %f", cost)
	}
}

func TestPricingUnknownModel(t *testing.T) {
	cost := CalculateCost("unknown-model", 1000000, 1000000)
	// unknown models use conservative estimate
	if cost == 0 {
		t.Error("expected unknown models to have non-zero cost")
	}
}
