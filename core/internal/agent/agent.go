package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kadet/kora/internal/llm"
	"github.com/kadet/kora/internal/logger"
	"github.com/kadet/kora/internal/session"
	"github.com/kadet/koramem"
)

func New(model, extractor llm.LLM, memory *koramem.Store, essencePath string) *Agent {
	systemPrompt := loadSystemPrompt(essencePath)

	return &Agent{
		llm:          model,
		extractor:    extractor,
		memory:       memory,
		sessions:     session.NewStore(),
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
	sess.AddMessage("user", userMessage)

	prompt := a.buildPrompt(userMessage)

	logger.Debug("calling llm", "messages", len(sess.Messages()))
	response, err := a.llm.Chat(ctx, prompt, sess.Messages())
	if err != nil {
		logger.Error("llm failed", "error", err)
		return "", err
	}

	logger.Debug("llm response", "chars", len(response))
	sess.AddMessage("assistant", response)

	go a.remember(ctx, sessionID, sess.Messages())

	return response, nil
}

func (a *Agent) buildPrompt(userMessage string) string {
	facts := a.recall(userMessage)

	if len(facts) == 0 {
		logger.Debug("no facts recalled")
		return a.systemPrompt
	}

	logger.Debug("facts recalled", "count", len(facts))

	var sb strings.Builder
	sb.WriteString(a.systemPrompt)
	sb.WriteString("\n\n## Recalled Memory\n")

	for _, f := range facts {
		fmt.Fprintf(&sb, "- %s: %s\n", f.Field, f.Value)
	}

	return sb.String()
}

func (a *Agent) recall(message string) []*koramem.Fact {
	allDomains := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}

	words := strings.Fields(message)
	var allFacts []*koramem.Fact
	seen := make(map[int64]bool)

	for _, word := range words {
		if len(word) < 3 {
			continue
		}

		facts, err := a.memory.SearchFacts(word, allDomains)
		if err != nil {
			continue
		}

		for _, f := range facts {
			if !seen[f.ID] {
				seen[f.ID] = true
				allFacts = append(allFacts, f)
			}
		}
	}

	return allFacts
}
