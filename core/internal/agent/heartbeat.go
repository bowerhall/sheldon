package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kadet/kora/internal/llm"
	"github.com/kadet/kora/internal/logger"
)

const heartbeatPrompt = `You are Kora, a personal AI assistant. Based on the user's stored context and current time, craft a brief, natural check-in message.

Current time: %s

Guidelines:
- Adapt your message to the time of day (morning greeting vs evening check-in)
- If it's morning and you know their wake time, consider a brief summary of today's priorities
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

	relevantDomains := []int{10, 12, 13} // goals, routines, events
	result, err := a.memory.Recall(ctx, "active goals current tasks recent events sleep wake schedule", relevantDomains, 10)
	if err != nil {
		logger.Error("heartbeat recall failed", "error", err)
		return "", err
	}

	var contextBuilder strings.Builder
	if len(result.Facts) > 0 {
		for _, f := range result.Facts {
			fmt.Fprintf(&contextBuilder, "- %s: %s\n", f.Field, f.Value)
		}
	} else {
		contextBuilder.WriteString("(No stored context yet)")
	}

	currentTime := time.Now().In(a.timezone).Format("Monday, January 2, 2006 3:04 PM")
	prompt := fmt.Sprintf(heartbeatPrompt, currentTime, contextBuilder.String())

	response, err := a.llm.Chat(ctx, a.systemPrompt, []llm.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		logger.Error("heartbeat llm failed", "error", err)
		return "", err
	}

	logger.Debug("heartbeat generated", "chars", len(response))

	sess := a.sessions.Get(sessionID)
	sess.AddMessage("assistant", response, nil, "")

	return response, nil
}
