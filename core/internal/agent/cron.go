package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/cron"
	"github.com/bowerhall/sheldon/internal/logger"
	"github.com/bowerhall/sheldonmem"
)

// CronRunner checks for due crons and triggers the agent loop
type CronRunner struct {
	crons    *cron.Store
	memory   *sheldonmem.Store
	trigger  TriggerFunc // injects into agent loop
	notify   NotifyFunc  // sends messages to chat
	timezone *time.Location
}

// NewCronRunner creates a new CronRunner
func NewCronRunner(crons *cron.Store, memory *sheldonmem.Store, trigger TriggerFunc, notify NotifyFunc, tz *time.Location) *CronRunner {
	return &CronRunner{
		crons:    crons,
		memory:   memory,
		trigger:  trigger,
		notify:   notify,
		timezone: tz,
	}
}

// Run starts the cron checker loop
func (r *CronRunner) Run(ctx context.Context) {
	// check every 10 seconds to support sub-minute schedules
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// initial check after short delay
	time.Sleep(5 * time.Second)
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
	// search memory for keyword across all domains
	result, err := r.memory.Recall(ctx, c.Keyword, nil, 10)
	if err != nil {
		logger.Error("cron memory recall failed", "keyword", c.Keyword, "error", err)
		return
	}

	// build context from recalled facts
	var factsContext strings.Builder
	if len(result.Facts) > 0 {
		for _, f := range result.Facts {
			fmt.Fprintf(&factsContext, "- %s: %s\n", f.Field, f.Value)
		}
	} else {
		factsContext.WriteString("(No specific context found)")
	}

	// format current time
	currentTime := time.Now().In(r.timezone).Format("Monday, January 2, 2006 3:04 PM")

	// build the trigger prompt
	prompt := fmt.Sprintf(`[SCHEDULED TRIGGER]
Keyword: %s
Current time: %s

Recalled context:
%s
This is a scheduled trigger you set up earlier. Take appropriate action based on the keyword and context:
- If keyword is "heartbeat" or "check-in": Send a brief, natural check-in message
- If keyword relates to a reminder (meds, water, stretch, etc.): Send a friendly reminder
- If keyword relates to a task (build-*, deploy-*, etc.): Start working on the task and report progress

Respond naturally - the user will see your message.`, c.Keyword, currentTime, factsContext.String())

	// inject into agent loop
	sessionID := fmt.Sprintf("telegram:%d", c.ChatID)
	response, err := r.trigger(c.ChatID, sessionID, prompt)
	if err != nil {
		logger.Error("cron trigger failed", "keyword", c.Keyword, "error", err)
		// still update next_run so we don't keep failing
	} else {
		// send response to chat
		if r.notify != nil && response != "" {
			r.notify(c.ChatID, response)
		}
		logger.Debug("cron fired", "keyword", c.Keyword, "chat", c.ChatID)
	}

	// calculate next run
	nextRun, err := r.crons.ComputeNextRun(c.Schedule)
	if err != nil {
		logger.Error("failed to compute next run", "schedule", c.Schedule, "error", err)
		return
	}

	if err := r.crons.UpdateNextRun(c.ID, nextRun); err != nil {
		logger.Error("failed to update cron next_run", "id", c.ID, "error", err)
	}

	logger.Debug("cron next run scheduled", "keyword", c.Keyword, "next", nextRun)
}
