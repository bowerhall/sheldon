package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/bowerhall/sheldon/internal/cron"
	"github.com/bowerhall/sheldon/internal/logger"
	"github.com/bowerhall/sheldonmem"
)

// CronRunner checks for due crons and fires reminders
type CronRunner struct {
	crons    *cron.Store
	memory   *sheldonmem.Store
	notify   NotifyFunc
	timezone *time.Location
}

// NewCronRunner creates a new CronRunner
func NewCronRunner(crons *cron.Store, memory *sheldonmem.Store, notify NotifyFunc, tz *time.Location) *CronRunner {
	return &CronRunner{
		crons:    crons,
		memory:   memory,
		notify:   notify,
		timezone: tz,
	}
}

// Run starts the cron checker loop
func (r *CronRunner) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	// initial check after short delay
	time.Sleep(10 * time.Second)
	r.checkDueCrons(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Debug("cron runner stopping")
			return
		case <-ticker.C:
			r.checkDueCrons(ctx)
		}
	}
}

func (r *CronRunner) checkDueCrons(ctx context.Context) {
	// cleanup expired crons
	deleted, err := r.crons.DeleteExpired()
	if err != nil {
		logger.Error("failed to delete expired crons", "error", err)
	} else if deleted > 0 {
		logger.Info("expired crons deleted", "count", deleted)
	}

	// get due crons
	crons, err := r.crons.GetDue()
	if err != nil {
		logger.Error("failed to get due crons", "error", err)
		return
	}

	for _, c := range crons {
		r.fireCron(ctx, c)
	}
}

func (r *CronRunner) fireCron(ctx context.Context, c cron.Cron) {
	// search memory for keyword
	result, err := r.memory.Recall(ctx, c.Keyword, []int{10, 12, 13}, 5)
	if err != nil {
		logger.Error("cron memory recall failed", "keyword", c.Keyword, "error", err)
		return
	}

	// format message from facts
	message := r.formatReminder(c.Keyword, result.Facts)

	// send notification
	if r.notify != nil {
		r.notify(c.ChatID, message)
		logger.Debug("cron fired", "keyword", c.Keyword, "chat", c.ChatID)
	}

	// calculate next run
	nextRun, err := cron.ComputeNextRun(c.Schedule)
	if err != nil {
		logger.Error("failed to compute next run", "schedule", c.Schedule, "error", err)
		return
	}

	if err := r.crons.UpdateNextRun(c.ID, nextRun); err != nil {
		logger.Error("failed to update cron next_run", "id", c.ID, "error", err)
	}

	logger.Debug("cron next run scheduled", "keyword", c.Keyword, "next", nextRun)
}

func (r *CronRunner) formatReminder(keyword string, facts []*sheldonmem.Fact) string {
	if len(facts) == 0 {
		return fmt.Sprintf("⏰ Reminder: %s", keyword)
	}

	// use the most relevant fact's value
	fact := facts[0]
	return fmt.Sprintf("⏰ %s", fact.Value)
}
