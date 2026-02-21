package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/bowerhall/sheldon/internal/logger"
	"github.com/bowerhall/sheldonmem"
	"github.com/robfig/cron/v3"
)

// CronRunner checks for due crons and fires reminders
type CronRunner struct {
	memory   *sheldonmem.Store
	notify   NotifyFunc
	timezone *time.Location
}

// NewCronRunner creates a new CronRunner
func NewCronRunner(memory *sheldonmem.Store, notify NotifyFunc, tz *time.Location) *CronRunner {
	return &CronRunner{
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
	deleted, err := r.memory.DeleteExpiredCrons()
	if err != nil {
		logger.Error("failed to delete expired crons", "error", err)
	} else if deleted > 0 {
		logger.Info("expired crons deleted", "count", deleted)
	}

	// get due crons
	crons, err := r.memory.GetDueCrons()
	if err != nil {
		logger.Error("failed to get due crons", "error", err)
		return
	}

	for _, c := range crons {
		r.fireCron(ctx, c)
	}
}

func (r *CronRunner) fireCron(ctx context.Context, c sheldonmem.Cron) {
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
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(c.Schedule)
	if err != nil {
		logger.Error("failed to parse cron schedule", "schedule", c.Schedule, "error", err)
		return
	}

	nextRun := sched.Next(time.Now())
	if err := r.memory.UpdateCronNextRun(c.ID, nextRun); err != nil {
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
