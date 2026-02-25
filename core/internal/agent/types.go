package agent

import (
	"time"

	"github.com/bowerhall/sheldon/internal/alerts"
	"github.com/bowerhall/sheldon/internal/budget"
	"github.com/bowerhall/sheldon/internal/config"
	"github.com/bowerhall/sheldon/internal/conversation"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/session"
	"github.com/bowerhall/sheldon/internal/tools"
	"github.com/bowerhall/sheldonmem"
)

type NotifyFunc func(chatID int64, message string)

// ProcessOptions configures how a message is processed
type ProcessOptions struct {
	Media   []llm.MediaContent
	Trusted bool // if true, sensitive facts are accessible; if false, SafeMode is enabled
}

// TriggerFunc processes a system trigger through the agent loop and returns the response
type TriggerFunc func(chatID int64, sessionID string, prompt string) (string, error)

// LLMFactory creates a new LLM instance based on current runtime config
type LLMFactory func() (llm.LLM, error)

type Agent struct {
	llm          llm.LLM
	extractor    llm.LLM
	memory       *sheldonmem.Store
	convo        *conversation.Store
	sessions     *session.Store
	tools        *tools.Registry
	systemPrompt string
	timezone     *time.Location
	notify       NotifyFunc
	budget       *budget.Tracker
	alerts       *alerts.Alerter
	skillsDir    string

	llmFactory    LLMFactory
	runtimeConfig *config.RuntimeConfig
	lastLLMHash   string
}

func (a *Agent) SetSkillsDir(dir string) {
	a.skillsDir = dir
}

func (a *Agent) SetConversationStore(store *conversation.Store) {
	a.convo = store
}
