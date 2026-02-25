package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/bowerhall/sheldon/internal/llm"
)

func RegisterTimeTools(registry *Registry, timezone *time.Location) {
	if timezone == nil {
		timezone = time.UTC
	}

	timeTool := llm.Tool{
		Name:        "current_time",
		Description: "Get the current date and time. Use this when you need to know what time it is, what day it is, or to calculate relative dates for memory searches.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	registry.Register(timeTool, func(ctx context.Context, args string) (string, error) {
		now := time.Now().In(timezone)
		_, week := now.ISOWeek()

		return fmt.Sprintf(`Current time: %s
Date: %s
Day: %s
Week: %d
Timezone: %s`,
			now.Format("15:04:05"),
			now.Format("2006-01-02"),
			now.Format("Monday"),
			week,
			timezone.String(),
		), nil
	})
}
