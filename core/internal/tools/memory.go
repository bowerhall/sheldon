package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldonmem"
)

type RecallArgs struct {
	Query     string `json:"query"`
	Domains   []int  `json:"domains,omitempty"`
	Depth     int    `json:"depth,omitempty"`
	TimeRange string `json:"time_range,omitempty"` // e.g., "today", "yesterday", "this_week", "last_week", "this_month"
}

type SaveMemoryArgs struct {
	Subject    string  `json:"subject,omitempty"`
	Field      string  `json:"field"`
	Value      string  `json:"value"`
	Domain     string  `json:"domain,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Sensitive  bool    `json:"sensitive,omitempty"`
}

type MarkSensitiveArgs struct {
	Field     string `json:"field"`
	Sensitive bool   `json:"sensitive"`
}


func RegisterMemoryTools(registry *Registry, memory *sheldonmem.Store) {
	// save_memory tool - explicit memory storage
	saveTool := llm.Tool{
		Name: "save_memory",
		Description: `Save a specific fact to long-term memory.

IMPORTANT: Only use this tool when the user EXPLICITLY asks you to remember something.
Examples of when to use:
- "Remember that my cat's name is Luna" (subject: user)
- "Sheldon, remember you should use more humor" (subject: sheldon)
- "Save this: I'm allergic to peanuts" (subject: user)
- "Don't forget you promised to be more concise" (subject: sheldon)

Do NOT use this tool:
- During normal conversation (facts are auto-extracted)
- When user mentions something casually without asking you to remember
- For information you're inferring rather than being told directly

The user must signal intent to save with words like "remember", "save", "don't forget", "note that", etc.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"subject": map[string]any{
					"type":        "string",
					"description": "Who is this fact about? 'user' for facts about the user, 'sheldon' for facts about yourself (behavior, style, promises). Default: user",
				},
				"field": map[string]any{
					"type":        "string",
					"description": "Short key for the fact (e.g., 'cat_name', 'allergy', 'communication_style')",
				},
				"value": map[string]any{
					"type":        "string",
					"description": "The information to remember",
				},
				"domain": map[string]any{
					"type":        "string",
					"description": "Category: identity, health, mind, beliefs, knowledge, relationships, career, finances, place, goals, preferences, routines, events, patterns. Default: knowledge",
				},
				"confidence": map[string]any{
					"type":        "number",
					"description": "How certain is this fact? 0.0-1.0. Use 1.0 for explicit user statements. Default: 1.0",
				},
				"sensitive": map[string]any{
					"type":        "boolean",
					"description": "Mark as sensitive (passwords, tokens, private keys, financial details). Sensitive facts are excluded when recalling during web browsing to prevent prompt injection extraction. Default: false",
				},
			},
			"required": []string{"field", "value"},
		},
	}

	registry.Register(saveTool, func(ctx context.Context, args string) (string, error) {
		var params SaveMemoryArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		domain := params.Domain
		if domain == "" {
			domain = "knowledge"
		}

		domainID, ok := sheldonmem.DomainSlugToID[domain]
		if !ok {
			domainID = 5 // default to knowledge
		}

		confidence := params.Confidence
		if confidence <= 0 {
			confidence = 1.0 // explicit saves are high confidence
		}

		// determine which entity to save to
		subject := strings.ToLower(params.Subject)
		var entity *sheldonmem.Entity
		var err error

		if subject == "sheldon" || subject == "self" || subject == "assistant" {
			entity, err = memory.FindEntityByName("Sheldon")
			if err != nil {
				return "", fmt.Errorf("could not find Sheldon entity: %w", err)
			}
		} else {
			// default to user
			entityName := UserEntityName(ctx)
			entity, err = memory.FindEntityByName(entityName)
			if err != nil {
				return "", fmt.Errorf("could not find user entity: %w", err)
			}
		}

		var result *sheldonmem.FactResult
		result, err = memory.AddFactWithContext(ctx, &entity.ID, domainID, params.Field, params.Value, confidence, params.Sensitive)
		if err != nil {
			return "", fmt.Errorf("failed to save: %w", err)
		}

		subjectLabel := "user"
		if subject == "sheldon" || subject == "self" || subject == "assistant" {
			subjectLabel = "self"
		}

		sensitiveLabel := ""
		if params.Sensitive {
			sensitiveLabel = " [SENSITIVE]"
		}

		if result.Superseded != nil {
			return fmt.Sprintf("Updated (%s): %s = %s (was: %s)%s", subjectLabel, params.Field, params.Value, result.Superseded.Value, sensitiveLabel), nil
		}

		return fmt.Sprintf("Saved (%s): %s = %s%s", subjectLabel, params.Field, params.Value, sensitiveLabel), nil
	})

	recallTool := llm.Tool{
		Name:        "recall_memory",
		Description: "Search your memory for relevant facts about the user. Use this when you need to remember something about the user's preferences, history, relationships, or any personal information they've shared.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "What to search for in memory (e.g., 'favorite food', 'wife's name', 'work schedule')",
				},
				"domains": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "integer"},
					"description": "Optional domain IDs to search (1=Identity, 2=Health, 3=Emotions, 4=Beliefs, 5=Skills, 6=Relationships, 7=Work, 8=Finances, 9=Location, 10=Goals, 11=Preferences, 12=Routines, 13=Events, 14=Patterns). Omit to search all.",
				},
				"depth": map[string]any{
					"type":        "integer",
					"description": "Graph traversal depth (1=direct connections, 2=friends of friends, 3=deeper). Use higher depth when exploring relationships or need broader context. Default: 1.",
				},
				"time_range": map[string]any{
					"type":        "string",
					"enum":        []string{"today", "yesterday", "this_week", "last_week", "this_month", "last_month"},
					"description": "Filter memories by time period. Use when user asks about recent events or specific time frames like 'yesterday', 'last week', etc.",
				},
			},
			"required": []string{"query"},
		},
	}

	registry.Register(recallTool, func(ctx context.Context, args string) (string, error) {
		var params RecallArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		domains := params.Domains
		if len(domains) == 0 {
			domains = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}
		} else {
			// validate domain IDs are in valid range (1-14)
			for _, d := range domains {
				if d < 1 || d > 14 {
					return "", fmt.Errorf("invalid domain ID %d: must be between 1 and 14", d)
				}
			}
		}

		depth := params.Depth
		if depth < 1 {
			depth = 1
		}

		opts := sheldonmem.RecallOptions{
			Depth:            depth,
			ExcludeSensitive: SafeModeFromContext(ctx),
		}

		// Apply time range filter if specified
		if params.TimeRange != "" {
			since, until := parseTimeRange(params.TimeRange)
			opts.Since = since
			opts.Until = until
		}

		result, err := memory.RecallWithOptions(ctx, params.Query, domains, 10, opts)
		if err != nil {
			return "", err
		}

		if len(result.Facts) == 0 && len(result.Entities) == 0 {
			return "No relevant memories found.", nil
		}

		var sb strings.Builder

		if len(result.Facts) > 0 {
			sb.WriteString("Facts:\n")
			for _, f := range result.Facts {
				sensitiveMarker := ""
				if f.Sensitive {
					sensitiveMarker = " [SENSITIVE]"
				}
				fmt.Fprintf(&sb, "- %s: %s%s\n", f.Field, f.Value, sensitiveMarker)

				// check for superseded (contradicting) values
				superseded, _ := memory.GetSupersededFacts(f.Field, f.EntityID)
				if len(superseded) > 0 {
					for _, old := range superseded {
						fmt.Fprintf(&sb, "  ↳ (previously: %s, changed %s)\n", old.Value, old.CreatedAt.Format("Jan 2"))
					}
				}
			}
		}

		if len(result.Entities) > 0 {
			sb.WriteString("\nRelated entities:\n")
			for _, t := range result.Entities {
				if t.Relation != "" {
					fmt.Fprintf(&sb, "- %s (%s, via %s, depth %d)\n", t.Entity.Name, t.Entity.EntityType, t.Relation, t.Depth)
				} else {
					fmt.Fprintf(&sb, "- %s (%s)\n", t.Entity.Name, t.Entity.EntityType)
				}
				for _, f := range t.Facts {
					fmt.Fprintf(&sb, "    • %s: %s\n", f.Field, f.Value)
				}
			}
		}

		return sb.String(), nil
	})

	markSensitiveTool := llm.Tool{
		Name:        "mark_sensitive",
		Description: "Mark an existing fact as sensitive or not sensitive. Sensitive facts are protected from prompt injection - they won't be revealed when processing web content.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"field": map[string]any{
					"type":        "string",
					"description": "The field name of the fact to mark (e.g., 'api_key', 'password')",
				},
				"sensitive": map[string]any{
					"type":        "boolean",
					"description": "Whether to mark as sensitive (true) or not sensitive (false)",
				},
			},
			"required": []string{"field", "sensitive"},
		},
	}

	registry.Register(markSensitiveTool, func(ctx context.Context, args string) (string, error) {
		var params MarkSensitiveArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		entityName := UserEntityName(ctx)
		entity, err := memory.FindEntityByName(entityName)
		if err != nil {
			return "", fmt.Errorf("could not find user entity: %w", err)
		}

		facts, err := memory.GetFactsByEntity(entity.ID)
		if err != nil {
			return "", fmt.Errorf("could not get facts: %w", err)
		}

		var found *sheldonmem.Fact
		for _, f := range facts {
			if f.Field == params.Field {
				found = f
				break
			}
		}

		if found == nil {
			return fmt.Sprintf("No fact found with field '%s'", params.Field), nil
		}

		if err := memory.MarkSensitive(found.ID, params.Sensitive); err != nil {
			return "", fmt.Errorf("failed to update: %w", err)
		}

		if params.Sensitive {
			return fmt.Sprintf("Marked '%s' as sensitive - it will be protected from prompt injection", params.Field), nil
		}
		return fmt.Sprintf("Marked '%s' as not sensitive", params.Field), nil
	})
}

// parseTimeRange converts a time range string to Since/Until times
func parseTimeRange(timeRange string) (*time.Time, *time.Time) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var since, until time.Time

	switch timeRange {
	case "today":
		since = today
		until = today.Add(24 * time.Hour)
	case "yesterday":
		since = today.Add(-24 * time.Hour)
		until = today
	case "this_week":
		// Start of this week (Monday)
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		since = today.Add(-time.Duration(weekday-1) * 24 * time.Hour)
		until = now
	case "last_week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		thisWeekStart := today.Add(-time.Duration(weekday-1) * 24 * time.Hour)
		since = thisWeekStart.Add(-7 * 24 * time.Hour)
		until = thisWeekStart
	case "this_month":
		since = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		until = now
	case "last_month":
		thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		since = thisMonthStart.AddDate(0, -1, 0)
		until = thisMonthStart
	default:
		return nil, nil
	}

	return &since, &until
}
