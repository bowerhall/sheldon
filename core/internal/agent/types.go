package agent

import (
	"time"

	"github.com/kadet/kora/internal/alerts"
	"github.com/kadet/kora/internal/budget"
	"github.com/kadet/kora/internal/llm"
	"github.com/kadet/kora/internal/session"
	"github.com/kadet/kora/internal/tools"
	"github.com/kadet/koramem"
)

type NotifyFunc func(chatID int64, message string)

type Contradiction struct {
	Field    string
	OldValue string
	NewValue string
}

type Agent struct {
	llm          llm.LLM
	extractor    llm.LLM
	memory       *koramem.Store
	sessions     *session.Store
	tools        *tools.Registry
	systemPrompt string
	timezone     *time.Location
	notify       NotifyFunc
	budget       *budget.Tracker
	alerts       *alerts.Alerter
	skillsDir    string
}

func (a *Agent) SetSkillsDir(dir string) {
	a.skillsDir = dir
}
