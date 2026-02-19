package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kadet/kora/internal/coder"
	"github.com/kadet/kora/internal/llm"
	"github.com/kadet/koramem"
)

type CoderArgs struct {
	Task       string `json:"task"`
	Complexity string `json:"complexity,omitempty"`
}

func RegisterCoderTool(registry *Registry, bridge *coder.Bridge, memory *koramem.Store) {
	tool := llm.Tool{
		Name:        "write_code",
		Description: "Execute code generation tasks. Use this for writing scripts, building applications, creating files, or any task that requires writing and testing code. Runs in a sandboxed environment with read/write/execute capabilities.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "Natural language description of what to build or do. Be specific about requirements, language preferences, and expected output.",
				},
				"complexity": map[string]any{
					"type":        "string",
					"enum":        []string{"simple", "standard", "complex"},
					"description": "Task complexity: simple (one file, <5 min), standard (multi-file, <10 min), complex (full project, <20 min). Defaults to standard.",
				},
			},
			"required": []string{"task"},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		var params CoderArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		complexity := coder.Complexity(params.Complexity)
		if complexity == "" {
			complexity = coder.ComplexityStandard
		}

		memCtx := buildMemoryContext(ctx, memory)

		task := coder.Task{
			ID:         uuid.New().String()[:8],
			Prompt:     params.Task,
			Complexity: complexity,
			Context:    memCtx,
		}

		result, err := bridge.Execute(ctx, task)
		if err != nil {
			return "", err
		}

		return formatResult(result), nil
	})
}

func buildMemoryContext(ctx context.Context, memory *koramem.Store) *coder.MemoryContext {
	memCtx := &coder.MemoryContext{
		UserPreferences: make(map[string]string),
		Constraints: []string{
			"Do not hardcode secrets or credentials",
			"Include error handling",
			"Keep code minimal and focused on the task",
		},
	}

	result, err := memory.Recall(ctx, "coding preferences language style deploy", []int{5, 7, 11}, 5)
	if err != nil {
		return memCtx
	}

	for _, f := range result.Facts {
		switch {
		case strings.Contains(strings.ToLower(f.Field), "language"):
			memCtx.UserPreferences["language"] = f.Value
		case strings.Contains(strings.ToLower(f.Field), "style"):
			memCtx.UserPreferences["style"] = f.Value
		default:
			memCtx.RelevantFacts = append(memCtx.RelevantFacts, coder.Fact{
				Field: f.Field,
				Value: f.Value,
			})
		}
	}

	return memCtx
}

func formatResult(result *coder.Result) string {
	var sb strings.Builder

	if result.Error != "" {
		fmt.Fprintf(&sb, "Error: %s\n\n", result.Error)
	}

	if len(result.Files) > 0 {
		sb.WriteString("Files created:\n")
		for _, f := range result.Files {
			fmt.Fprintf(&sb, "- %s\n", f)
		}
		sb.WriteString("\n")
	}

	if result.Output != "" {
		sb.WriteString("Output:\n")
		sb.WriteString(result.Output)
		sb.WriteString("\n")
	}

	if result.Sanitized {
		sb.WriteString("\n⚠️ Some content was redacted for security.\n")
	}

	fmt.Fprintf(&sb, "\nCompleted in %s", result.Duration.Round(time.Second))

	return sb.String()
}
