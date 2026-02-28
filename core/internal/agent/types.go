package agent

import (
	"sync"
	"time"

	"github.com/bowerhall/sheldon/internal/alerts"
	"github.com/bowerhall/sheldon/internal/approval"
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
	Trusted bool  // if true, sensitive facts are accessible; if false, SafeMode is enabled
	UserID  int64 // ID of the user who sent the message (for approval verification)
}

// TriggerFunc processes a system trigger through the agent loop and returns the response
type TriggerFunc func(chatID int64, sessionID string, prompt string) (string, error)

// LLMFactory creates a new LLM instance based on current runtime config
type LLMFactory func() (llm.LLM, error)

// ApprovalSender sends approval request buttons to the user
type ApprovalSender func(chatID int64, message string, approvalID string) error

type Agent struct {
	mu           sync.RWMutex
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

	approvals      *approval.Manager
	approvalSender ApprovalSender
}

func (a *Agent) SetSkillsDir(dir string) {
	a.skillsDir = dir
}

func (a *Agent) SetConversationStore(store *conversation.Store) {
	a.convo = store
}

func (a *Agent) SetApprovalManager(mgr *approval.Manager) {
	a.approvals = mgr
}

func (a *Agent) SetApprovalSender(sender ApprovalSender) {
	a.approvalSender = sender
}
