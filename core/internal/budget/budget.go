package budget

import (
	"sync"
	"time"
)

type Tracker struct {
	mu         sync.Mutex
	dailyLimit int
	warnAt     float64
	tokens     int
	lastReset  time.Time
	onWarn     func(used, limit int)
	onExceeded func(used, limit int)
	warnSent   bool
	timezone   *time.Location
	store      *Store
}

type Config struct {
	DailyLimit int
	WarnAt     float64
	Timezone   *time.Location
}

func NewTracker(cfg Config, onWarn, onExceeded func(used, limit int)) *Tracker {
	tz := cfg.Timezone
	if tz == nil {
		tz = time.UTC
	}

	return &Tracker{
		dailyLimit: cfg.DailyLimit,
		warnAt:     cfg.WarnAt,
		lastReset:  time.Now().In(tz),
		onWarn:     onWarn,
		onExceeded: onExceeded,
		timezone:   tz,
	}
}

func (t *Tracker) SetStore(s *Store) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.store = s

	// load today's usage from persistent store
	if s != nil {
		if tokens, err := s.TodayTokens(); err == nil {
			t.tokens = tokens
			// check if we should have already warned
			if float64(t.tokens) >= float64(t.dailyLimit)*t.warnAt {
				t.warnSent = true
			}
		}
	}
}

func (t *Tracker) Store() *Store {
	return t.store
}

func (t *Tracker) Add(tokens int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.checkReset()
	t.tokens += tokens

	if t.tokens >= t.dailyLimit {
		if t.onExceeded != nil {
			t.onExceeded(t.tokens, t.dailyLimit)
		}

		return false
	}

	if !t.warnSent && float64(t.tokens) >= float64(t.dailyLimit)*t.warnAt {
		t.warnSent = true

		if t.onWarn != nil {
			t.onWarn(t.tokens, t.dailyLimit)
		}
	}

	return true
}

func (t *Tracker) Record(provider, model string, inputTokens, outputTokens int) bool {
	totalTokens := inputTokens + outputTokens

	if t.store != nil {
		if err := t.store.Record(provider, model, inputTokens, outputTokens); err != nil {
			// log but don't fail - usage tracking shouldn't block responses
			println("budget: failed to record usage:", err.Error())
		}
	}

	return t.Add(totalTokens)
}

func (t *Tracker) Usage() (used, limit int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.checkReset()
	return t.tokens, t.dailyLimit
}

// must hold lock
func (t *Tracker) checkReset() {
	now := time.Now().In(t.timezone)
	if now.YearDay() != t.lastReset.YearDay() || now.Year() != t.lastReset.Year() {
		t.tokens = 0
		t.warnSent = false
		t.lastReset = now
	}
}
