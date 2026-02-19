package alerts

import (
	"fmt"
	"sync"
	"time"

	"github.com/kadet/kora/internal/logger"
)

type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarn
	SeverityCritical
)

type NotifyFunc func(message string)

type Alerter struct {
	mu        sync.Mutex
	notify    NotifyFunc
	cooldowns map[string]time.Time
	cooldown  time.Duration
}

func New(notify NotifyFunc, cooldown time.Duration) *Alerter {
	return &Alerter{
		notify:    notify,
		cooldowns: make(map[string]time.Time),
		cooldown:  cooldown,
	}
}

func (a *Alerter) Alert(severity Severity, component, message string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := fmt.Sprintf("%s:%s", component, message)

	if lastSent, ok := a.cooldowns[key]; ok {
		if time.Since(lastSent) < a.cooldown {
			logger.Debug("alert suppressed (cooldown)", "component", component, "message", message)
			return
		}
	}

	var text string
	switch severity {
	case SeverityCritical:
		text = fmt.Sprintf("🚨 %s: %s", component, message)
	case SeverityWarn:
		text = fmt.Sprintf("⚠️ %s: %s", component, message)
	default:
		text = fmt.Sprintf("ℹ️ %s: %s", component, message)
	}

	if err != nil {
		text += fmt.Sprintf("\n\nError: %v", err)
	}

	if a.notify != nil {
		a.notify(text)
		a.cooldowns[key] = time.Now()
		logger.Info("alert sent", "component", component, "severity", severity)
	}
}

func (a *Alerter) Critical(component, message string, err error) {
	a.Alert(SeverityCritical, component, message, err)
}

func (a *Alerter) Warn(component, message string, err error) {
	a.Alert(SeverityWarn, component, message, err)
}
