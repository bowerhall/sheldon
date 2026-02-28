package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

// fallbackProviders is the priority order for automatic failover
// ollama is last resort - only used if a local model is already installed
var fallbackProviders = []string{"kimi", "claude", "openai", "ollama"}

const defaultMaxToolIterations = 20
const maxToolFailures = 3

// maxToolIterations is configurable via AGENT_MAX_ITERATIONS env var
var maxToolIterations = defaultMaxToolIterations

func init() {
	if v := os.Getenv("AGENT_MAX_ITERATIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxToolIterations = n
		}
	}
}

func New(model, extractor llm.LLM, memory *sheldonmem.Store, essencePath, timezone string) *Agent {
	systemPrompt := loadSystemPrompt(essencePath)

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		logger.Warn("invalid timezone, using UTC", "timezone", timezone, "error", err)
		loc = time.UTC
	}

	registry := tools.NewRegistry()
	tools.RegisterMemoryTools(registry, memory)
	tools.RegisterNoteTools(registry, memory)
	tools.RegisterTimeTools(registry, loc)

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
	// Force immediate refresh to sync with runtime config
	a.lastLLMHash = ""
	if err := a.refreshLLMIfNeeded(); err != nil {
		logger.Warn("failed to refresh LLM on factory setup", "error", err)
	}
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

	a.setLLM(newLLM)
	a.lastLLMHash = currentHash
	logger.Info("LLM instance refreshed", "config", currentHash)

	return nil
}

func (a *Agent) getLLM() llm.LLM {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.llm
}

func (a *Agent) setLLM(model llm.LLM) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.llm = model
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

// buildDynamicPrompt adds dynamic context (like active notes) to the system prompt
func (a *Agent) buildDynamicPrompt() string {
	prompt := a.systemPrompt

	// Add active notes with age to context
	notes, err := a.memory.ListNotesWithAge()
	if err == nil && len(notes) > 0 {
		var parts []string
		for _, n := range notes {
			age := formatAge(n.UpdatedAt)
			parts = append(parts, fmt.Sprintf("%s (%s)", n.Key, age))
		}
		prompt += fmt.Sprintf("\n\n## Active Notes\n%s", strings.Join(parts, ", "))
	}

	return prompt
}

// formatAge returns a human-readable age string like "2 days ago" or "3 hours ago"
func formatAge(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins <= 1 {
			return "just now"
		}
		return fmt.Sprintf("%d mins ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		weeks := int(d.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	}
}

func (a *Agent) Process(ctx context.Context, sessionID string, userMessage string) (string, error) {
	return a.ProcessWithOptions(ctx, sessionID, userMessage, ProcessOptions{Trusted: true})
}

func (a *Agent) ProcessWithMedia(ctx context.Context, sessionID string, userMessage string, media []llm.MediaContent) (string, error) {
	return a.ProcessWithOptions(ctx, sessionID, userMessage, ProcessOptions{Media: media, Trusted: true})
}

func (a *Agent) ProcessWithOptions(ctx context.Context, sessionID string, userMessage string, opts ProcessOptions) (string, error) {
	media := opts.Media
	logger.Debug("message received", "session", sessionID, "media", len(media))

	if err := a.refreshLLMIfNeeded(); err != nil {
		logger.Warn("failed to refresh LLM, using existing instance", "error", err)
	}

	// Check model capabilities for media
	caps := a.getLLM().Capabilities()
	hasImage := false
	hasVideo := false
	hasPDF := false
	for _, m := range media {
		if m.Type == llm.MediaTypeImage {
			hasImage = true
		}
		if m.Type == llm.MediaTypeVideo {
			hasVideo = true
		}
		if m.Type == llm.MediaTypePDF {
			hasPDF = true
		}
	}

	// Keep original media for tools, but filter for LLM based on capabilities
	mediaForLLM := media
	var limitations []string

	if hasImage && !caps.Vision {
		limitations = append(limitations, "image")
	}
	if hasVideo && !caps.VideoInput {
		limitations = append(limitations, "video")
	}
	if hasPDF && !caps.PDFInput {
		limitations = append(limitations, "PDF")
	}

	if len(limitations) > 0 {
		note := fmt.Sprintf("[%s received but current model doesn't support %s analysis. I can still save it for you.]",
			strings.Join(limitations, " and "), strings.Join(limitations, "/"))
		if userMessage == "" {
			userMessage = note
		} else {
			userMessage = note + " " + userMessage
		}

		// Filter unsupported media types
		mediaForLLM = nil
		for _, m := range media {
			if m.Type == llm.MediaTypeImage && caps.Vision {
				mediaForLLM = append(mediaForLLM, m)
			}
			if m.Type == llm.MediaTypeVideo && caps.VideoInput {
				mediaForLLM = append(mediaForLLM, m)
			}
			if m.Type == llm.MediaTypePDF && caps.PDFInput {
				mediaForLLM = append(mediaForLLM, m)
			}
		}
	}

	sess := a.sessions.Get(sessionID)
	chatID := a.parseChatID(sessionID)

	// prevent concurrent processing of same session
	if !sess.TryAcquire() {
		logger.Debug("session busy, queueing message", "session", sessionID)
		sess.Queue(userMessage, media, opts.Trusted)
		return "", nil // no response - typing indicator shows we're busy
	}
	defer func() {
		sess.Release()
		// process any queued messages
		a.processQueue(ctx, sessionID, sess, chatID)
	}()

	// load recent conversation history for continuity
	if len(sess.Messages()) == 0 && a.convo != nil {
		recent, err := a.convo.GetRecent(sessionID)
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
				logger.Info("loading recent conversation", "session", sessionID, "messages", len(loaded), "skipped", startIdx)
				for _, m := range loaded {
					sess.AddMessage(m.Role, m.Content, nil, "")
				}
			}
		} else {
			logger.Debug("no recent messages found", "session", sessionID)
		}
	} else if a.convo == nil {
		logger.Warn("conversation store not configured")
	}

	if len(sess.Messages()) == 0 && a.isNewUser(sessionID) {
		logger.Info("new user detected, triggering interview", "session", sessionID)
		sess.AddMessage("system", "[This is a new user with no stored memory. Start with a warm welcome and begin the setup interview to get to know them. Follow the interview guide in your instructions.]", nil, "")
	}

	sess.AddMessageWithMedia("user", userMessage, mediaForLLM, nil, "")

	// check for skill command (e.g., /apartment-hunter)
	if skill := a.detectSkillCommand(userMessage); skill != "" {
		skillContent := a.loadSkill(skill)
		if skillContent != "" {
			sess.AddMessage("system", fmt.Sprintf("[Skill activated: %s]\n\n%s", skill, skillContent), nil, "")
			logger.Debug("skill activated", "skill", skill)
		}
	}

	// add session info to context for tools
	ctx = context.WithValue(ctx, tools.ChatIDKey, chatID)
	ctx = context.WithValue(ctx, tools.SessionIDKey, sessionID)
	if opts.UserID != 0 {
		ctx = context.WithValue(ctx, tools.UserIDKey, opts.UserID)
	}
	if len(media) > 0 {
		ctx = context.WithValue(ctx, tools.MediaKey, media)
	}
	// SafeMode excludes sensitive facts - enabled when not trusted
	if !opts.Trusted {
		ctx = context.WithValue(ctx, tools.SafeModeKey, true)
	}

	response, err := a.runAgentLoop(ctx, sess)
	if err != nil {
		logger.Error("agent loop failed", "error", err)
		return "", err
	}

	// save to recent conversation buffer
	if a.convo != nil {
		if _, err := a.convo.Add(sessionID, "user", userMessage); err != nil {
			logger.Warn("failed to save user message to conversation buffer", "error", err)
		}
		if result, err := a.convo.Add(sessionID, "assistant", response); err != nil {
			logger.Warn("failed to save assistant response to conversation buffer", "error", err)
		} else if result != nil && len(result.Overflow) > 0 {
			// Buffer overflowed - save evicted messages as a chunk for daily summaries
			go a.saveOverflowAsChunk(sessionID, result.Overflow)
		}
	}

	// extract facts only from the new exchange (not the buffer)
	go a.rememberExchange(ctx, sessionID, userMessage, response)

	// check if we need to generate summaries for previous days (async)
	go a.generatePendingSummaries(ctx, sessionID)

	return response, nil
}

// processQueue handles any messages that were queued while we were busy
func (a *Agent) processQueue(ctx context.Context, sessionID string, sess *session.Session, chatID int64) {
	msg := sess.Dequeue()
	if msg == nil {
		return
	}

	logger.Info("processing queued message", "session", sessionID, "remaining", sess.QueueLen())

	// process in background so we don't block
	go func() {
		response, err := a.ProcessWithOptions(ctx, sessionID, msg.Content, ProcessOptions{
			Media:   msg.Media,
			Trusted: msg.Trusted,
		})
		if err != nil {
			logger.Error("failed to process queued message", "error", err)
			return
		}
		if response != "" && a.notify != nil {
			a.notify(chatID, response)
		}
	}()
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

// browserTools trigger isolated mode - they process untrusted external content
var browserTools = map[string]bool{
	"browse":       true,
	"browse_click": true,
	"browse_fill":  true,
	"search_web":   true,
}

// degradedModeTools are safe, read-only tools allowed when running on local fallback model
var degradedModeTools = map[string]bool{
	"recall_memory":   true,
	"current_time":    true,
	"usage_summary":   true,
	"usage_breakdown": true,
	"current_model":   true,
	"list_providers":  true,
	"list_crons":      true,
	"read_note":       true,
	"list_notes":      true,
}

func (a *Agent) runAgentLoop(ctx context.Context, sess *session.Session) (string, error) {
	availableTools := a.tools.Tools()
	toolFailures := make(map[string]int)    // track consecutive failures per tool
	failedProviders := make(map[string]bool) // track providers that failed this request
	isolatedMode := false                    // restrict tools after browse/code to prevent prompt injection
	degradedMode := false                    // restrict to safe tools when on local fallback

	for i := range maxToolIterations {
		// filter tools based on mode
		loopTools := availableTools
		if degradedMode {
			loopTools = filterDegradedTools(availableTools)
		} else if isolatedMode {
			loopTools = filterIsolatedTools(availableTools)
		}

		// get current LLM (may change during fallback)
		currentLLM := a.getLLM()

		logger.Debug("agent loop iteration", "iteration", i, "messages", len(sess.Messages()), "isolatedMode", isolatedMode, "degradedMode", degradedMode)

		resp, err := currentLLM.ChatWithTools(ctx, a.buildDynamicPrompt(), sess.Messages(), loopTools)
		if err != nil {
			// try fallback provider if quota exhausted
			if shouldFallback(err) {
				currentProvider := currentLLM.Provider()
				failedProviders[currentProvider] = true
				logger.Warn("provider unavailable, trying fallback", "provider", currentProvider, "error", err, "failedProviders", failedProviders)

				newLLM, newProvider, fallbackErr := a.tryFallbackProvider(ctx, failedProviders)
				if fallbackErr != nil {
					if a.alerts != nil {
						a.alerts.Critical("llm", "All providers exhausted", err)
					}
					return fmt.Sprintf("%s is unavailable and no fallback providers are configured. Please try again later or add another provider (KIMI_API_KEY, OPENAI_API_KEY).", currentProvider), nil
				}

				// switch to fallback - enter degraded mode if using local model
				a.setLLM(newLLM)
				if newProvider == "ollama" {
					degradedMode = true
					logger.Info("entered degraded mode", "provider", newProvider)
				}
				logger.Info("switched to fallback provider", "from", currentProvider, "to", newProvider)
				continue // retry with new provider
			}

			if a.alerts != nil {
				a.alerts.Critical("llm", "Chat request failed", err)
			}
			return "", err
		}

		if resp.Usage != nil && a.budget != nil {
			logger.Info("recording usage", "provider", currentLLM.Provider(), "model", currentLLM.Model(), "input", resp.Usage.PromptTokens, "output", resp.Usage.CompletionTokens)
			if !a.budget.Record(currentLLM.Provider(), currentLLM.Model(), resp.Usage.PromptTokens, resp.Usage.CompletionTokens) {
				return "I've reached my daily API limit. Please try again tomorrow!", nil
			}
		} else {
			logger.Warn("skipping usage recording", "hasUsage", resp.Usage != nil, "hasBudget", a.budget != nil)
		}

		if len(resp.ToolCalls) == 0 {
			logger.Info("llm response (no tools)", "chars", len(resp.Content))
			sess.AddMessage("assistant", resp.Content, nil, "")
			return resp.Content, nil
		}

		logger.Info("llm requested tools", "count", len(resp.ToolCalls))
		sess.AddMessage("assistant", resp.Content, resp.ToolCalls, "")

		for _, tc := range resp.ToolCalls {
			logger.Info("executing tool", "name", tc.Name, "isolatedMode", isolatedMode)

			var result string
			var err error

			// check if tool requires approval
			if tools.RequiresApproval(tc.Name) && a.approvals != nil && a.approvalSender != nil {
				chatID := tools.ChatIDFromContext(ctx)
				userID := tools.UserIDFromContext(ctx)

				desc := a.describeToolCall(tc.Name, tc.Arguments)
				approvalID := a.approvals.Start(chatID, userID, tc.Name, tc.Arguments, desc)

				sendErr := a.approvalSender(chatID, desc, approvalID)
				if sendErr != nil {
					a.approvals.Cancel(approvalID)
					result = fmt.Sprintf("Failed to request approval: %s", sendErr.Error())
				} else {
					approved, approvalErr := a.approvals.Wait(ctx, approvalID)
					if approvalErr != nil {
						result = fmt.Sprintf("Approval request failed: %s", approvalErr.Error())
					} else if !approved {
						result = fmt.Sprintf("User denied %s (approval %s)", tc.Name, approvalID)
						logger.Info("tool denied by user", "tool", tc.Name, "approvalID", approvalID)
					} else {
						logger.Info("tool approved by user", "tool", tc.Name, "approvalID", approvalID)
						result, err = a.tools.Execute(ctx, tc.Name, tc.Arguments)
					}
				}
			} else {
				result, err = a.tools.Execute(ctx, tc.Name, tc.Arguments)
			}

			// enter isolated mode after browser tools to prevent prompt injection
			if browserTools[tc.Name] {
				isolatedMode = true
				logger.Info("entered isolated mode", "trigger", tc.Name)
			}
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

// tools disabled during isolated operations (browse/code) to prevent prompt injection attacks
// isolated mode is read-only: no state changes allowed after processing untrusted content
var disabledDuringIsolation = map[string]bool{
	// data extraction
	"recall_memory": true,

	// data poisoning
	"save_memory":    true,
	"mark_sensitive": true,
	"save_note":      true,
	"delete_note":    true,
	"archive_note":   true,
	"restore_note":   true,

	// config changes
	"set_config":    true,
	"reset_config":  true,
	"switch_model":  true,
	"pull_model":    true,
	"remove_model":  true,

	// scheduled tasks
	"set_cron":    true,
	"delete_cron": true,
	"pause_cron":  true,
	"resume_cron": true,

	// code & deployment
	"write_code":  true,
	"deploy_app":  true,
	"remove_app":  true,
	"build_image": true,

	// skills
	"install_skill": true,
	"save_skill":    true,
	"remove_skill":  true,

	// file operations
	"upload_file": true,
	"delete_file": true,
	"save_media":  true,

	// external actions
	"open_pr":     true,
	"create_repo": true,
	"send_image":  true,
	"send_video":  true,

	// container management
	"start_container":   true,
	"stop_container":    true,
	"restart_container": true,
}

func filterIsolatedTools(tools []llm.Tool) []llm.Tool {
	filtered := make([]llm.Tool, 0, len(tools))
	for _, t := range tools {
		if !disabledDuringIsolation[t.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func filterDegradedTools(tools []llm.Tool) []llm.Tool {
	filtered := make([]llm.Tool, 0, len(tools))
	for _, t := range tools {
		if degradedModeTools[t.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
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

// shouldFallback checks if an error warrants switching to another provider
// Triggers on: quota/credit issues, overloaded servers, rate limits
func shouldFallback(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// quota/credit errors
	if strings.Contains(errStr, "credit") ||
		strings.Contains(errStr, "quota") ||
		strings.Contains(errStr, "insufficient") ||
		strings.Contains(errStr, "402") ||
		strings.Contains(errStr, "payment required") ||
		strings.Contains(errStr, "billing") ||
		strings.Contains(errStr, "exceeded") {
		return true
	}
	// overloaded/rate limit errors
	if strings.Contains(errStr, "overloaded") ||
		strings.Contains(errStr, "529") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "429") {
		return true
	}
	return false
}

// tryFallbackProvider attempts to switch to another configured provider
// For ollama: only uses already-installed models, never pulls new ones
func (a *Agent) tryFallbackProvider(ctx context.Context, failedProviders map[string]bool) (llm.LLM, string, error) {
	if a.runtimeConfig == nil {
		return nil, "", fmt.Errorf("no runtime config available")
	}

	for _, provider := range fallbackProviders {
		if failedProviders[provider] {
			continue
		}

		var apiKey, model, baseURL string

		if provider == "ollama" {
			// check for installed local model (don't pull new ones)
			model = a.findInstalledOllamaModel(ctx)
			if model == "" {
				logger.Debug("no installed ollama models for fallback")
				continue
			}
			baseURL = a.runtimeConfig.Get("ollama_host")
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			apiKey = "ollama"
		} else {
			// cloud provider - check if API key is configured
			envKey := config.EnvKeyForProvider(provider)
			if envKey == "" || os.Getenv(envKey) == "" {
				continue
			}
			apiKey = os.Getenv(envKey)
			model = defaultModelForProvider(provider)
		}

		// try to create LLM for this provider
		newLLM, err := llm.New(llm.Config{
			Provider: provider,
			APIKey:   apiKey,
			Model:    model,
			BaseURL:  baseURL,
		})
		if err != nil {
			logger.Warn("failed to create fallback LLM", "provider", provider, "error", err)
			continue
		}

		// update runtime config
		a.runtimeConfig.Set("llm_provider", provider)
		a.runtimeConfig.Set("llm_model", model)
		a.lastLLMHash = provider + ":" + model

		logger.Info("switched to fallback provider", "provider", provider, "model", model)
		return newLLM, provider, nil
	}

	return nil, "", fmt.Errorf("no fallback providers available")
}

// default ollama models to try for fallback (in preference order)
var defaultOllamaFallbackModels = []string{
	"llama3.2", "llama3.1", "llama3",
	"qwen2.5:7b", "qwen2.5:3b", "qwen2.5",
	"mistral", "gemma2",
}

// findInstalledOllamaModel checks for a suitable chat model already installed in ollama
func (a *Agent) findInstalledOllamaModel(ctx context.Context) string {
	ollamaHost := a.runtimeConfig.Get("ollama_host")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}

	// check for custom preference via env
	preferred := defaultOllamaFallbackModels
	if custom := os.Getenv("OLLAMA_FALLBACK_MODELS"); custom != "" {
		preferred = strings.Split(custom, ",")
		for i := range preferred {
			preferred[i] = strings.TrimSpace(preferred[i])
		}
	}

	// get installed models via ollama API
	resp, err := http.Get(ollamaHost + "/api/tags")
	if err != nil {
		logger.Debug("ollama not reachable", "error", err)
		return ""
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	installed := make(map[string]bool)
	for _, m := range result.Models {
		installed[m.Name] = true
		// also index without tag for partial matching
		if idx := strings.Index(m.Name, ":"); idx > 0 {
			installed[m.Name[:idx]] = true
		}
	}

	// return first preferred model that's installed
	for _, pref := range preferred {
		if installed[pref] {
			// find exact name with tag
			for _, m := range result.Models {
				if m.Name == pref || strings.HasPrefix(m.Name, pref+":") {
					return m.Name
				}
			}
		}
	}

	// fallback: return any installed model (skip embeddings)
	for _, m := range result.Models {
		if !strings.Contains(m.Name, "embed") {
			return m.Name
		}
	}

	return ""
}

func defaultModelForProvider(provider string) string {
	switch provider {
	case "kimi":
		return "kimi-k2-0711-preview"
	case "claude":
		return "claude-sonnet-4-20250514"
	case "openai":
		return "gpt-4o"
	default:
		return ""
	}
}

func (a *Agent) describeToolCall(toolName, args string) string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		return fmt.Sprintf("[Approval Required]\nTool: %s\nArgs: %s", toolName, args)
	}

	switch toolName {
	case "deploy_app":
		name, _ := parsed["name"].(string)
		if name == "" {
			name = "unknown"
		}
		return fmt.Sprintf("[Approval Required]\nTool: deploy_app\nAction: Deploy \"%s\" to production", name)
	case "remove_app":
		name, _ := parsed["name"].(string)
		if name == "" {
			name = "unknown"
		}
		return fmt.Sprintf("[Approval Required]\nTool: remove_app\nAction: Remove \"%s\" from production", name)
	default:
		return fmt.Sprintf("[Approval Required]\nTool: %s", toolName)
	}
}
