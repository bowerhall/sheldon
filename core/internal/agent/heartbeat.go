package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/kadet/kora/internal/llm"
	"github.com/kadet/kora/internal/logger"
)

const heartbeatPrompt = `You are Kora, a personal AI assistant. Based on the user's stored context below, craft a brief, natural check-in message.

Guidelines:
- If there are active goals or tasks, ask about progress on the most relevant one
- If nothing urgent, just say a casual hi or ask what they're up to
- Keep it short (1-2 sentences)
- Be warm but not overly enthusiastic
- Don't mention that you're checking stored facts

User context:
%s

Craft your check-in message:`

func (a *Agent) Heartbeat(ctx context.Context, sessionID string) (string, error) {
	logger.Debug("heartbeat triggered", "session", sessionID)

	// Recall goals (D10), routines (D12), recent events (D13)
	relevantDomains := []int{10, 12, 13}
	result, err := a.memory.Recall(ctx, "active goals current tasks recent events", relevantDomains, 10)
	if err != nil {
		logger.Error("heartbeat recall failed", "error", err)
		return "", err
	}

	// Format context
	var contextBuilder strings.Builder
	if len(result.Facts) > 0 {
		for _, f := range result.Facts {
			fmt.Fprintf(&contextBuilder, "- %s: %s\n", f.Field, f.Value)
		}
	} else {
		contextBuilder.WriteString("(No stored context yet)")
	}

	prompt := fmt.Sprintf(heartbeatPrompt, contextBuilder.String())

	// Generate check-in message
	response, err := a.llm.Chat(ctx, a.systemPrompt, []llm.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		logger.Error("heartbeat llm failed", "error", err)
		return "", err
	}

	logger.Debug("heartbeat generated", "chars", len(response))

	// Add to session so conversation flows naturally
	sess := a.sessions.Get(sessionID)
	sess.AddMessage("assistant", response, nil, "")

	return response, nil
}
