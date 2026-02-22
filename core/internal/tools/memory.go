package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldonmem"
)

type RecallArgs struct {
	Query   string `json:"query"`
	Domains []int  `json:"domains,omitempty"`
	Depth   int    `json:"depth,omitempty"`
}

type SaveMemoryArgs struct {
	Field      string  `json:"field"`
	Value      string  `json:"value"`
	Domain     string  `json:"domain,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

var domainNameToID = map[string]int{
	"identity":      1,
	"health":        2,
	"mind":          3,
	"beliefs":       4,
	"knowledge":     5,
	"relationships": 6,
	"career":        7,
	"finances":      8,
	"place":         9,
	"goals":         10,
	"preferences":   11,
	"routines":      12,
	"events":        13,
	"patterns":      14,
}

func RegisterMemoryTools(registry *Registry, memory *sheldonmem.Store) {
	// save_memory tool - explicit memory storage
	saveTool := llm.Tool{
		Name: "save_memory",
		Description: `Save a specific fact to long-term memory.

IMPORTANT: Only use this tool when the user EXPLICITLY asks you to remember something.
Examples of when to use:
- "Remember that my cat's name is Luna"
- "Save this: I'm allergic to peanuts"
- "Don't forget I have a meeting at 3pm tomorrow"
- "Please remember my anniversary is March 15th"

Do NOT use this tool:
- During normal conversation (facts are auto-extracted)
- When user mentions something casually without asking you to remember
- For information you're inferring rather than being told directly

The user must signal intent to save with words like "remember", "save", "don't forget", "note that", etc.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"field": map[string]any{
					"type":        "string",
					"description": "Short key for the fact (e.g., 'cat_name', 'allergy', 'anniversary')",
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

		domainID, ok := domainNameToID[domain]
		if !ok {
			domainID = 5 // default to knowledge
		}

		confidence := params.Confidence
		if confidence <= 0 {
			confidence = 1.0 // explicit saves are high confidence
		}

		// get user entity from context
		chatID := ChatIDFromContext(ctx)
		entityName := fmt.Sprintf("user_telegram_%d", chatID)

		entity, err := memory.FindEntityByName(entityName)
		if err != nil {
			return "", fmt.Errorf("could not find user entity: %w", err)
		}

		result, err := memory.AddFact(&entity.ID, domainID, params.Field, params.Value, confidence)
		if err != nil {
			return "", fmt.Errorf("failed to save: %w", err)
		}

		if result.Superseded != nil {
			return fmt.Sprintf("Updated: %s = %s (was: %s)", params.Field, params.Value, result.Superseded.Value), nil
		}

		return fmt.Sprintf("Saved: %s = %s", params.Field, params.Value), nil
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
		}

		depth := params.Depth
		if depth < 1 {
			depth = 1
		}

		opts := sheldonmem.RecallOptions{Depth: depth}
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
				fmt.Fprintf(&sb, "- %s: %s\n", f.Field, f.Value)

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
}
