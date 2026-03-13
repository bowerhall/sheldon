package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bowerhall/sheldon/internal/cron"
	"github.com/bowerhall/sheldon/internal/logger"
	"github.com/bowerhall/sheldonmem"
)

// CronRunner checks for due crons and triggers the agent loop
type CronRunner struct {
	crons              *cron.Store
	memory             *sheldonmem.Store
	trigger            TriggerFunc // injects into agent loop
	notify             NotifyFunc  // sends messages to chat
	timezone           *time.Location
	agent              *Agent    // for system crons
	mu                 sync.Mutex
	lastExtractionRun  time.Time // track last extraction run (every 6 hours)
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

// SetAgent sets the agent for system crons (end-of-day processing)
func (r *CronRunner) SetAgent(agent *Agent) {
	r.agent = agent
}

// Run starts the cron checker loop
func (r *CronRunner) Run(ctx context.Context) {
	// check every 10 seconds to support sub-minute schedules
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// initial check after short delay (context-aware)
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second):
	}
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
	// check system crons first
	r.checkSystemCrons(ctx)

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

// checkSystemCrons handles hardcoded system crons like memory extraction
func (r *CronRunner) checkSystemCrons(ctx context.Context) {
	if r.agent == nil {
		return
	}

	now := time.Now()

	r.mu.Lock()
	shouldRun := now.Sub(r.lastExtractionRun) >= 6*time.Hour
	if shouldRun {
		r.lastExtractionRun = now
	}
	r.mu.Unlock()

	// Memory extraction: runs every 6 hours, processes messages older than 6 hours
	// Failsafe: if runs were missed, all unprocessed messages get caught up
	if shouldRun {
		logger.Info("running memory extraction")
		extractCtx := context.WithoutCancel(ctx)
		go func() {
			if err := r.agent.ProcessEndOfDay(extractCtx, false); err != nil {
				logger.Error("memory extraction failed", "error", err)
			}
		}()
	}
}

func (r *CronRunner) fireCron(ctx context.Context, c cron.Cron) {
	sessionID := fmt.Sprintf("telegram:%d", c.ChatID)

	// HYBRID SEARCH: semantic on embedded facts + keyword on recent messages
	// This ensures same-day context (not yet embedded) is still found

	// 1. Semantic search on embedded facts
	result, err := r.memory.Recall(ctx, c.Keyword, nil, 10)
	if err != nil {
		logger.Error("cron memory recall failed", "keyword", c.Keyword, "error", err)
	}

	// 2. Keyword search on recent daily messages (catches same-day context)
	recentMsgs, err := r.memory.SearchRecentByKeyword(sessionID, c.Keyword, 2)
	if err != nil {
		logger.Error("cron daily search failed", "keyword", c.Keyword, "error", err)
	}

	// Build combined context
	var factsContext strings.Builder

	// Add semantic results
	if result != nil && len(result.Facts) > 0 {
		factsContext.WriteString("From memory:\n")
		for _, f := range result.Facts {
			fmt.Fprintf(&factsContext, "- %s: %s\n", f.Field, f.Value)
		}
	}

	// Add recent message context (only user messages)
	var userMsgs []sheldonmem.DailyMessage
	for _, m := range recentMsgs {
		if m.Role == "user" {
			userMsgs = append(userMsgs, m)
		}
	}
	if len(userMsgs) > 0 {
		if factsContext.Len() > 0 {
			factsContext.WriteString("\n")
		}
		factsContext.WriteString("From recent conversation:\n")
		for _, m := range userMsgs {
			fmt.Fprintf(&factsContext, "- User said: %s\n", truncate(m.Content, 200))
		}
	}

	if factsContext.Len() == 0 {
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
- If keyword is "checkin" or similar: Send a brief, natural check-in message
- If keyword relates to a reminder (meds, water, stretch, etc.): Send a friendly reminder
- If keyword relates to a task (build-*, deploy-*, etc.): Start working on the task and report progress

Respond naturally - the user will see your message.`, c.Keyword, currentTime, factsContext.String())

	// inject into agent loop
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

	// one-time crons: delete after firing instead of rescheduling
	// detected by expiry being set and before the next computed run
	if c.ExpiresAt != nil && c.ExpiresAt.Before(nextRun) {
		if err := r.crons.Delete(c.ID); err != nil {
			logger.Error("failed to delete one-time cron", "id", c.ID, "error", err)
		} else {
			logger.Debug("one-time cron fired and deleted", "keyword", c.Keyword)
		}
		return
	}

	if err := r.crons.UpdateNextRun(c.ID, nextRun); err != nil {
		logger.Error("failed to update cron next_run", "id", c.ID, "error", err)
	}

	logger.Debug("cron next run scheduled", "keyword", c.Keyword, "next", nextRun)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
