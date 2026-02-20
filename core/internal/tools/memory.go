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

func RegisterMemoryTools(registry *Registry, memory *sheldonmem.Store) {
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
