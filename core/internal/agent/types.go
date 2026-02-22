package agent

import (
	"time"

	"github.com/bowerhall/sheldon/internal/alerts"
	"github.com/bowerhall/sheldon/internal/budget"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/session"
	"github.com/bowerhall/sheldon/internal/tools"
	"github.com/bowerhall/sheldonmem"
)

type NotifyFunc func(chatID int64, message string)

// TriggerFunc processes a system trigger through the agent loop and returns the response
type TriggerFunc func(chatID int64, sessionID string, prompt string) (string, error)

type Agent struct {
	llm          llm.LLM
	extractor    llm.LLM
	memory       *sheldonmem.Store
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
