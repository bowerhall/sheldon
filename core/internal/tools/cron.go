package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/cron"
	"github.com/bowerhall/sheldon/internal/llm"
)

type SetCronArgs struct {
	Keyword   string `json:"keyword"`
	Schedule  string `json:"schedule"`
	ExpiresIn string `json:"expires_in,omitempty"`
}

type DeleteCronArgs struct {
	Keyword string `json:"keyword"`
}

type PauseCronArgs struct {
	Keyword string `json:"keyword"`
	Until   string `json:"until"`
}

func RegisterCronTools(registry *Registry, cronStore *cron.Store) {
	// set_cron tool
	setCronTool := llm.Tool{
		Name:        "set_cron",
		Description: "Schedule a recurring trigger. When the cron fires, you'll wake up with the keyword's recalled context and decide what to do: send a check-in, reminder, or start working on a task. Use 'heartbeat' keyword for periodic check-ins.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Short keyword for memory search when cron fires (e.g., 'heartbeat', 'meds', 'standup', 'build-weather-app')",
				},
				"schedule": map[string]any{
					"type":        "string",
					"description": "Cron expression: minute hour day-of-month month day-of-week. Examples: '0 20 * * *' (8pm daily), '0 9 * * 1-5' (9am weekdays), '0 */6 * * *' (every 6 hours), '0 15 22 2 *' (3pm on Feb 22)",
				},
				"expires_in": map[string]any{
					"type":        "string",
					"description": "When to auto-delete. Examples: '2 weeks', '1 month', '1 day'. Omit for permanent.",
				},
			},
			"required": []string{"keyword", "schedule"},
		},
	}

	registry.Register(setCronTool, func(ctx context.Context, args string) (string, error) {
		var params SetCronArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		chatID := ChatIDFromContext(ctx)
		if chatID == 0 {
			return "", fmt.Errorf("no chat context available")
		}

		var expiresAt *time.Time
		if params.ExpiresIn != "" {
			t := parseExpiry(params.ExpiresIn)
			if t != nil {
				expiresAt = t
			}
		}

		c, err := cronStore.Create(params.Keyword, params.Schedule, chatID, expiresAt)
		if err != nil {
			return "", fmt.Errorf("failed to create cron: %w", err)
		}

		expiryInfo := ""
		if expiresAt != nil {
			expiryInfo = fmt.Sprintf(" (expires %s)", expiresAt.Format("Jan 2, 2006"))
		}

		return fmt.Sprintf("Reminder '%s' scheduled. Next: %s%s",
			c.Keyword,
			c.NextRun.Format("Mon Jan 2 3:04 PM"),
			expiryInfo), nil
	})

	// list_crons tool
	listCronsTool := llm.Tool{
		Name:        "list_crons",
		Description: "List all active scheduled triggers for this chat",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	registry.Register(listCronsTool, func(ctx context.Context, args string) (string, error) {
		chatID := ChatIDFromContext(ctx)
		if chatID == 0 {
			return "", fmt.Errorf("no chat context available")
		}

		crons, err := cronStore.GetByChat(chatID)
		if err != nil {
			return "", fmt.Errorf("failed to list crons: %w", err)
		}

		if len(crons) == 0 {
			return "No active scheduled triggers.", nil
		}

		var sb strings.Builder
		sb.WriteString("Active scheduled triggers:\n")
		for _, c := range crons {
			status := ""
			if c.PausedUntil != nil && c.PausedUntil.After(time.Now()) {
				status = fmt.Sprintf(" [PAUSED until %s]", c.PausedUntil.Format("Mon Jan 2 3:04 PM"))
			}
			expiryInfo := ""
			if c.ExpiresAt != nil {
				expiryInfo = fmt.Sprintf(" (expires %s)", c.ExpiresAt.Format("Jan 2"))
			}
			fmt.Fprintf(&sb, "- %s: next %s, schedule '%s'%s%s\n",
				c.Keyword,
				c.NextRun.Format("Mon Jan 2 3:04 PM"),
				c.Schedule,
				status,
				expiryInfo)
		}
		return sb.String(), nil
	})

	// delete_cron tool
	deleteCronTool := llm.Tool{
		Name:        "delete_cron",
		Description: "Delete a scheduled trigger by its keyword",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Keyword of the trigger to delete",
				},
			},
			"required": []string{"keyword"},
		},
	}

	registry.Register(deleteCronTool, func(ctx context.Context, args string) (string, error) {
		var params DeleteCronArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		chatID := ChatIDFromContext(ctx)
		if chatID == 0 {
			return "", fmt.Errorf("no chat context available")
		}

		err := cronStore.DeleteByKeyword(params.Keyword, chatID)
		if err != nil {
			return "", fmt.Errorf("failed to delete cron: %w", err)
		}

		return fmt.Sprintf("Trigger '%s' deleted.", params.Keyword), nil
	})

	// pause_cron tool
	pauseCronTool := llm.Tool{
		Name:        "pause_cron",
		Description: "Temporarily pause a scheduled trigger until a specified time. Use this when the user wants you to 'go quiet' or 'stop checking in' for a while.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Keyword of the trigger to pause (e.g., 'heartbeat')",
				},
				"until": map[string]any{
					"type":        "string",
					"description": "When to resume. Examples: '3 hours', 'tomorrow morning', '2026-02-22 09:00'",
				},
			},
			"required": []string{"keyword", "until"},
		},
	}

	registry.Register(pauseCronTool, func(ctx context.Context, args string) (string, error) {
		var params PauseCronArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		chatID := ChatIDFromContext(ctx)
		if chatID == 0 {
			return "", fmt.Errorf("no chat context available")
		}

		// parse the until time
		until := parseExpiry(params.Until)
		if until == nil {
			return "", fmt.Errorf("could not parse time: %s", params.Until)
		}

		err := cronStore.SetPausedUntil(params.Keyword, chatID, until)
		if err != nil {
			return "", fmt.Errorf("failed to pause cron: %w", err)
		}

		return fmt.Sprintf("Trigger '%s' paused until %s.", params.Keyword, until.Format("Mon Jan 2 3:04 PM")), nil
	})

	// resume_cron tool (unpause)
	resumeCronTool := llm.Tool{
		Name:        "resume_cron",
		Description: "Resume a paused scheduled trigger immediately",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Keyword of the trigger to resume",
				},
			},
			"required": []string{"keyword"},
		},
	}

	registry.Register(resumeCronTool, func(ctx context.Context, args string) (string, error) {
		var params DeleteCronArgs // reuse struct, same shape
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		chatID := ChatIDFromContext(ctx)
		if chatID == 0 {
			return "", fmt.Errorf("no chat context available")
		}

		err := cronStore.SetPausedUntil(params.Keyword, chatID, nil)
		if err != nil {
			return "", fmt.Errorf("failed to resume cron: %w", err)
		}

		return fmt.Sprintf("Trigger '%s' resumed.", params.Keyword), nil
	})
}

// parseExpiry converts human-readable duration to time
func parseExpiry(s string) *time.Time {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "never" {
		return nil
	}

	var duration time.Duration

	// parse common patterns
	patterns := map[string]time.Duration{
		"1 day":    24 * time.Hour,
		"2 days":   2 * 24 * time.Hour,
		"3 days":   3 * 24 * time.Hour,
		"1 week":   7 * 24 * time.Hour,
		"2 weeks":  14 * 24 * time.Hour,
		"3 weeks":  21 * 24 * time.Hour,
		"1 month":  30 * 24 * time.Hour,
		"2 months": 60 * 24 * time.Hour,
		"3 months": 90 * 24 * time.Hour,
		"6 months": 180 * 24 * time.Hour,
		"1 year":   365 * 24 * time.Hour,
	}

	if d, ok := patterns[s]; ok {
		duration = d
	} else {
		// try parsing "N units" format
		var n int
		var unit string
		if _, err := fmt.Sscanf(s, "%d %s", &n, &unit); err == nil {
			unit = strings.TrimSuffix(unit, "s") // normalize plural
			switch unit {
			case "minute":
				duration = time.Duration(n) * time.Minute
			case "hour":
				duration = time.Duration(n) * time.Hour
			case "day":
				duration = time.Duration(n) * 24 * time.Hour
			case "week":
				duration = time.Duration(n) * 7 * 24 * time.Hour
			case "month":
				duration = time.Duration(n) * 30 * 24 * time.Hour
			case "year":
				duration = time.Duration(n) * 365 * 24 * time.Hour
			}
		}
	}

	if duration == 0 {
		return nil
	}

	t := time.Now().Add(duration)
	return &t
}
