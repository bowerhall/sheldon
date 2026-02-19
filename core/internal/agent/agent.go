package agent

import (
	"context"
	"os"
	"path/filepath"

	"github.com/kadet/kora/internal/llm"
	"github.com/kadet/kora/internal/logger"
	"github.com/kadet/kora/internal/session"
	"github.com/kadet/kora/internal/tools"
	"github.com/kadet/koramem"
)

const maxToolIterations = 5

func New(model, extractor llm.LLM, memory *koramem.Store, essencePath string) *Agent {
	systemPrompt := loadSystemPrompt(essencePath)

	registry := tools.NewRegistry()
	tools.RegisterMemoryTools(registry, memory)

	return &Agent{
		llm:          model,
		extractor:    extractor,
		memory:       memory,
		sessions:     session.NewStore(),
		tools:        registry,
		systemPrompt: systemPrompt,
	}
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
	logger.Debug("message received", "session", sessionID)

	sess := a.sessions.Get(sessionID)
	sess.AddMessage("user", userMessage, nil, "")

	response, err := a.runAgentLoop(ctx, sess)
	if err != nil {
		logger.Error("agent loop failed", "error", err)
		return "", err
	}

	go a.remember(ctx, sessionID, sess.Messages())

	return response, nil
}

func (a *Agent) runAgentLoop(ctx context.Context, sess *session.Session) (string, error) {
	availableTools := a.tools.Tools()

	for i := range maxToolIterations {
		logger.Debug("agent loop iteration", "iteration", i, "messages", len(sess.Messages()))

		resp, err := a.llm.ChatWithTools(ctx, a.systemPrompt, sess.Messages(), availableTools)
		if err != nil {
			return "", err
		}

		if len(resp.ToolCalls) == 0 {
			logger.Debug("llm response (final)", "chars", len(resp.Content))
			sess.AddMessage("assistant", resp.Content, nil, "")
			return resp.Content, nil
		}

		logger.Debug("llm requested tools", "count", len(resp.ToolCalls))
		sess.AddMessage("assistant", resp.Content, resp.ToolCalls, "")

		for _, tc := range resp.ToolCalls {
			logger.Debug("executing tool", "name", tc.Name, "id", tc.ID)

			result, err := a.tools.Execute(ctx, tc.Name, tc.Arguments)
			if err != nil {
				result = "Error: " + err.Error()
			}

			logger.Debug("tool result", "name", tc.Name, "chars", len(result))
			sess.AddMessage("tool", result, nil, tc.ID)
		}
	}

	logger.Warn("agent loop hit max iterations", "max", maxToolIterations)
	return "I apologize, but I'm having trouble completing this request. Please try again.", nil
}
