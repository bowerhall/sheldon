package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/alerts"
	"github.com/bowerhall/sheldon/internal/budget"
	"github.com/bowerhall/sheldon/internal/config"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/logger"
	"github.com/bowerhall/sheldon/internal/session"
	"github.com/bowerhall/sheldon/internal/tools"
	"github.com/bowerhall/sheldonmem"
)

const maxToolIterations = 10
const maxToolFailures = 3

func New(model, extractor llm.LLM, memory *sheldonmem.Store, essencePath, timezone string) *Agent {
	systemPrompt := loadSystemPrompt(essencePath)

	registry := tools.NewRegistry()
	tools.RegisterMemoryTools(registry, memory)

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		logger.Warn("invalid timezone, using UTC", "timezone", timezone, "error", err)
		loc = time.UTC
	}

	return &Agent{
		llm:          model,
		extractor:    extractor,
		memory:       memory,
		sessions:     session.NewStore(),
		tools:        registry,
		systemPrompt: systemPrompt,
		timezone:     loc,
	}
}

func (a *Agent) SetNotifyFunc(fn NotifyFunc) {
	a.notify = fn
	// also wire up notifications to tool registry
	a.tools.SetNotify(tools.NotifyFunc(fn))
}

func (a *Agent) SetBudget(b *budget.Tracker) {
	a.budget = b
}

func (a *Agent) SetAlerter(alerter *alerts.Alerter) {
	a.alerts = alerter
}

func (a *Agent) SetLLMFactory(factory LLMFactory, rc *config.RuntimeConfig) {
	a.llmFactory = factory
	a.runtimeConfig = rc
	a.lastLLMHash = a.currentLLMHash()
}

func (a *Agent) currentLLMHash() string {
	if a.runtimeConfig == nil {
		return ""
	}
	provider := a.runtimeConfig.Get("llm_provider")
	model := a.runtimeConfig.Get("llm_model")
	return provider + ":" + model
}

func (a *Agent) refreshLLMIfNeeded() error {
	if a.llmFactory == nil || a.runtimeConfig == nil {
		return nil
	}

	currentHash := a.currentLLMHash()
	if currentHash == a.lastLLMHash {
		return nil
	}

	newLLM, err := a.llmFactory()
	if err != nil {
		logger.Error("failed to create new LLM instance", "error", err)
		return err
	}

	a.llm = newLLM
	a.lastLLMHash = currentHash
	logger.Info("LLM instance refreshed", "config", currentHash)

	return nil
}

func (a *Agent) Registry() *tools.Registry {
	return a.tools
}

func (a *Agent) Memory() *sheldonmem.Store {
	return a.memory
}

func loadSystemPrompt(essencePath string) string {
	soulPath := filepath.Join(essencePath, "SOUL.md")
	soul, err := os.ReadFile(soulPath)
	if err != nil {
		return ""
	}

	return string(soul)
}

func (a *Agent) Process(ctx context.Context, sessionID string, userMessage string) (string, error) {
	return a.ProcessWithImages(ctx, sessionID, userMessage, nil)
}

func (a *Agent) ProcessWithImages(ctx context.Context, sessionID string, userMessage string, images []llm.ImageContent) (string, error) {
	logger.Debug("message received", "session", sessionID, "images", len(images))

	if err := a.refreshLLMIfNeeded(); err != nil {
		logger.Warn("failed to refresh LLM, using existing instance", "error", err)
	}

	sess := a.sessions.Get(sessionID)
	chatID := a.parseChatID(sessionID)

	// prevent concurrent processing of same session
	if !sess.TryAcquire() {
		logger.Debug("session busy, queueing message", "session", sessionID)
		return "I'm still working on your previous request. I'll get to this once I'm done!", nil
	}
	defer sess.Release()

	// load recent conversation history for continuity
	if len(sess.Messages()) == 0 && a.convo != nil {
		recent, err := a.convo.GetRecent(chatID)
		if err != nil {
			logger.Warn("failed to load recent messages", "error", err)
		} else if len(recent) > 0 {
			// skip leading assistant messages - conversation must start with user
			startIdx := 0
			for startIdx < len(recent) && recent[startIdx].Role != "user" {
				startIdx++
			}
			if startIdx < len(recent) {
				loaded := recent[startIdx:]
				logger.Info("loading recent conversation", "chatID", chatID, "messages", len(loaded), "skipped", startIdx)
				for _, m := range loaded {
					sess.AddMessage(m.Role, m.Content, nil, "")
				}
			}
		} else {
			logger.Debug("no recent messages found", "chatID", chatID)
		}
	} else if a.convo == nil {
		logger.Warn("conversation store not configured")
	}

	if len(sess.Messages()) == 0 && a.isNewUser(sessionID) {
		logger.Info("new user detected, triggering interview", "session", sessionID)
		sess.AddMessage("system", "[This is a new user with no stored memory. Start with a warm welcome and begin the setup interview to get to know them. Follow the interview guide in your instructions.]", nil, "")
	}

	sess.AddMessageWithImages("user", userMessage, images, nil, "")

	// check for skill command (e.g., /apartment-hunter)
	if skill := a.detectSkillCommand(userMessage); skill != "" {
		skillContent := a.loadSkill(skill)
		if skillContent != "" {
			sess.AddMessage("system", fmt.Sprintf("[Skill activated: %s]\n\n%s", skill, skillContent), nil, "")
			logger.Debug("skill activated", "skill", skill)
		}
	}

	// add chatID to context for tool notifications
	ctx = context.WithValue(ctx, tools.ChatIDKey, chatID)

	response, err := a.runAgentLoop(ctx, sess)
	if err != nil {
		logger.Error("agent loop failed", "error", err)
		return "", err
	}

	// save to recent conversation buffer
	if a.convo != nil {
		if err := a.convo.Add(chatID, "user", userMessage); err != nil {
			logger.Warn("failed to save user message to conversation buffer", "error", err)
		}
		if err := a.convo.Add(chatID, "assistant", response); err != nil {
			logger.Warn("failed to save assistant response to conversation buffer", "error", err)
		}
	}

	// extract facts only from the new exchange (not the buffer)
	go a.rememberExchange(ctx, sessionID, userMessage, response)

	return response, nil
}

func (a *Agent) parseChatID(sessionID string) int64 {
	// format: "telegram:123456" or "discord:123456"
	parts := strings.Split(sessionID, ":")
	if len(parts) != 2 {
		return 0
	}
	id, _ := strconv.ParseInt(parts[1], 10, 64)
	return id
}

func (a *Agent) isNewUser(sessionID string) bool {
	entityID := a.getOrCreateUserEntity(sessionID)
	if entityID == 0 {
		return true
	}

	facts, err := a.memory.GetFactsByEntity(entityID)
	return err != nil || len(facts) == 0
}

func (a *Agent) detectSkillCommand(message string) string {
	message = strings.TrimSpace(message)
	if !strings.HasPrefix(message, "/") {
		return ""
	}

	// extract command name (first word after /)
	parts := strings.Fields(message)
	if len(parts) == 0 {
		return ""
	}

	cmd := strings.TrimPrefix(parts[0], "/")
	if cmd == "" {
		return ""
	}

	// check if skill exists
	if a.skillsDir == "" {
		return ""
	}

	skillPath := filepath.Join(a.skillsDir, strings.ToUpper(cmd)+".md")
	if _, err := os.Stat(skillPath); err == nil {
		return cmd
	}

	return ""
}

func (a *Agent) loadSkill(name string) string {
	if a.skillsDir == "" {
		return ""
	}

	skillPath := filepath.Join(a.skillsDir, strings.ToUpper(name)+".md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		return ""
	}

	return string(content)
}

func (a *Agent) runAgentLoop(ctx context.Context, sess *session.Session) (string, error) {
	availableTools := a.tools.Tools()
	toolFailures := make(map[string]int) // track consecutive failures per tool

	for i := range maxToolIterations {
		logger.Debug("agent loop iteration", "iteration", i, "messages", len(sess.Messages()))

		resp, err := a.llm.ChatWithTools(ctx, a.systemPrompt, sess.Messages(), availableTools)
		if err != nil {
			if a.alerts != nil {
				a.alerts.Critical("llm", "Chat request failed", err)
			}
			return "", err
		}

		if resp.Usage != nil && a.budget != nil {
			if !a.budget.Add(resp.Usage.TotalTokens) {
				return "I've reached my daily API limit. Please try again tomorrow!", nil
			}
		}

		if len(resp.ToolCalls) == 0 {
			logger.Info("llm response (no tools)", "chars", len(resp.Content))
			sess.AddMessage("assistant", resp.Content, nil, "")
			return resp.Content, nil
		}

		logger.Info("llm requested tools", "count", len(resp.ToolCalls))
		sess.AddMessage("assistant", resp.Content, resp.ToolCalls, "")

		for _, tc := range resp.ToolCalls {
			logger.Info("executing tool", "name", tc.Name)

			result, err := a.tools.Execute(ctx, tc.Name, tc.Arguments)
			if err != nil {
				toolFailures[tc.Name]++
				logger.Warn("tool execution failed", "name", tc.Name, "error", err, "failures", toolFailures[tc.Name])

				// circuit breaker: if same tool fails 3 times, abort with clear feedback
				if toolFailures[tc.Name] >= maxToolFailures {
					errorMsg := fmt.Sprintf("I tried using '%s' %d times but it kept failing. Last error: %s. I'm stopping to avoid spinning in circles. Please check the issue or try a different approach.", tc.Name, maxToolFailures, err.Error())
					logger.Error("circuit breaker triggered", "tool", tc.Name, "failures", toolFailures[tc.Name])
					sess.AddMessage("tool", errorMsg, nil, tc.ID)
					return errorMsg, nil
				}

				// provide clear, actionable error message
				result = fmt.Sprintf("[TOOL ERROR] %s failed (attempt %d/%d): %s", tc.Name, toolFailures[tc.Name], maxToolFailures, err.Error())
			} else {
				// reset failure count on success
				toolFailures[tc.Name] = 0
			}

			logger.Debug("tool result", "name", tc.Name, "chars", len(result))
			sess.AddMessage("tool", result, nil, tc.ID)
		}
	}

	logger.Warn("agent loop hit max iterations", "max", maxToolIterations)
	return "I apologize, but I'm having trouble completing this request. Please try again.", nil
}

// ProcessSystemTrigger handles a scheduled trigger (cron-based). Unlike user messages,
// system triggers don't wait for session locks - they run in their own context.
// This allows crons to fire even when a conversation is in progress.
func (a *Agent) ProcessSystemTrigger(ctx context.Context, sessionID string, triggerPrompt string) (string, error) {
	logger.Debug("system trigger received", "session", sessionID)

	sess := a.sessions.Get(sessionID)

	// Add trigger as a system message so the agent knows this isn't a user speaking
	sess.AddMessage("user", triggerPrompt, nil, "")

	// Add chatID to context for tool access
	chatID := a.parseChatID(sessionID)
	ctx = context.WithValue(ctx, tools.ChatIDKey, chatID)

	response, err := a.runAgentLoop(ctx, sess)
	if err != nil {
		logger.Error("system trigger processing failed", "error", err)
		return "", err
	}

	// system triggers don't extract facts - they're internal prompts, not user input

	return response, nil
}
