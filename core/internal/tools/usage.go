package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/budget"
	"github.com/bowerhall/sheldon/internal/llm"
)

func RegisterUsageTools(registry *Registry, store *budget.Store, timezone *time.Location) {
	if store == nil {
		return
	}

	usageTool := llm.Tool{
		Name:        "usage_summary",
		Description: "Get a summary of API usage and costs. Shows total requests, tokens, and cost in USD for a time period.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"period": map[string]any{
					"type":        "string",
					"enum":        []string{"today", "week", "month", "custom"},
					"description": "Time period: today, week (this week), month (this month), or custom (requires from/to)",
				},
				"from": map[string]any{
					"type":        "string",
					"description": "Start date for custom period (YYYY-MM-DD)",
				},
				"to": map[string]any{
					"type":        "string",
					"description": "End date for custom period (YYYY-MM-DD)",
				},
			},
			"required": []string{"period"},
		},
	}

	registry.Register(usageTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Period string `json:"period"`
			From   string `json:"from"`
			To     string `json:"to"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		var summary *budget.Summary
		var periodLabel string
		var err error

		switch params.Period {
		case "today":
			summary, err = store.Today()
			periodLabel = "today"
		case "week":
			summary, err = store.ThisWeek()
			periodLabel = "this week"
		case "month":
			summary, err = store.ThisMonth()
			periodLabel = "this month"
		case "custom":
			from, err := time.ParseInLocation("2006-01-02", params.From, timezone)
			if err != nil {
				return "", fmt.Errorf("invalid from date: %w", err)
			}
			to, err := time.ParseInLocation("2006-01-02", params.To, timezone)
			if err != nil {
				return "", fmt.Errorf("invalid to date: %w", err)
			}
			to = to.Add(24 * time.Hour) // include the end date
			summary, err = store.SummaryRange(from, to)
			if err != nil {
				return "", err
			}
			periodLabel = fmt.Sprintf("%s to %s", params.From, params.To)
		default:
			return "", fmt.Errorf("invalid period: %s", params.Period)
		}

		if err != nil {
			return "", err
		}

		return fmt.Sprintf(
			"Usage for %s:\n- Requests: %d\n- Input tokens: %d\n- Output tokens: %d\n- Total cost: $%.4f",
			periodLabel,
			summary.TotalRequests,
			summary.TotalInputTokens,
			summary.TotalOutputTokens,
			summary.TotalCostUSD,
		), nil
	})

	breakdownTool := llm.Tool{
		Name:        "usage_breakdown",
		Description: "Get a detailed breakdown of API usage by model or by day. Returns data that can be formatted as a table.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"by": map[string]any{
					"type":        "string",
					"enum":        []string{"model", "day"},
					"description": "Group by model or by day",
				},
				"period": map[string]any{
					"type":        "string",
					"enum":        []string{"today", "week", "month", "custom"},
					"description": "Time period",
				},
				"from": map[string]any{
					"type":        "string",
					"description": "Start date for custom period (YYYY-MM-DD)",
				},
				"to": map[string]any{
					"type":        "string",
					"description": "End date for custom period (YYYY-MM-DD)",
				},
			},
			"required": []string{"by", "period"},
		},
	}

	registry.Register(breakdownTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			By     string `json:"by"`
			Period string `json:"period"`
			From   string `json:"from"`
			To     string `json:"to"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		now := time.Now().In(timezone)
		var from, to time.Time

		switch params.Period {
		case "today":
			from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timezone)
			to = from.Add(24 * time.Hour)
		case "week":
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			from = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, timezone)
			to = from.Add(7 * 24 * time.Hour)
		case "month":
			from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, timezone)
			to = from.AddDate(0, 1, 0)
		case "custom":
			var err error
			from, err = time.ParseInLocation("2006-01-02", params.From, timezone)
			if err != nil {
				return "", fmt.Errorf("invalid from date: %w", err)
			}
			to, err = time.ParseInLocation("2006-01-02", params.To, timezone)
			if err != nil {
				return "", fmt.Errorf("invalid to date: %w", err)
			}
			to = to.Add(24 * time.Hour)
		default:
			return "", fmt.Errorf("invalid period: %s", params.Period)
		}

		var result strings.Builder

		switch params.By {
		case "model":
			breakdown, err := store.BreakdownByModel(from, to)
			if err != nil {
				return "", err
			}
			if len(breakdown) == 0 {
				return "No usage data for this period.", nil
			}
			result.WriteString("| Model | Requests | Input Tokens | Output Tokens | Cost |\n")
			result.WriteString("|-------|----------|--------------|---------------|------|\n")
			for _, b := range breakdown {
				result.WriteString(fmt.Sprintf("| %s | %d | %d | %d | $%.4f |\n",
					b.Model, b.Requests, b.InputTokens, b.OutputTokens, b.CostUSD))
			}
		case "day":
			breakdown, err := store.BreakdownByDay(from, to)
			if err != nil {
				return "", err
			}
			if len(breakdown) == 0 {
				return "No usage data for this period.", nil
			}
			result.WriteString("| Date | Requests | Input Tokens | Output Tokens | Cost |\n")
			result.WriteString("|------|----------|--------------|---------------|------|\n")
			for _, b := range breakdown {
				result.WriteString(fmt.Sprintf("| %s | %d | %d | %d | $%.4f |\n",
					b.Date, b.Requests, b.InputTokens, b.OutputTokens, b.CostUSD))
			}
		default:
			return "", fmt.Errorf("invalid breakdown type: %s", params.By)
		}

		return result.String(), nil
	})
}
