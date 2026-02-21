package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/coder"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldonmem"
	"github.com/google/uuid"
)

type CoderArgs struct {
	Task       string `json:"task"`
	Complexity string `json:"complexity,omitempty"`
	GitRepo    string `json:"git_repo,omitempty"` // target repo name (e.g., "weather-bot")
}

func RegisterCoderTool(registry *Registry, bridge *coder.Bridge, memory *sheldonmem.Store) {
	tool := llm.Tool{
		Name:        "write_code",
		Description: "Execute code generation tasks. Use this for writing scripts, building applications, creating files, or any task that requires writing and testing code. Runs in a sandboxed environment with read/write/execute capabilities. If git_repo is specified, code will be committed incrementally and pushed to that repo in the configured org.",
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
				"git_repo": map[string]any{
					"type":        "string",
					"description": "Target repository name for the code (e.g., 'weather-bot'). If specified, commits will be pushed to GIT_ORG_URL/git_repo. Repo will be created if it doesn't exist.",
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

		// notify user that coding has started
		taskSummary := params.Task
		if len(taskSummary) > 50 {
			taskSummary = taskSummary[:50] + "..."
		}
		registry.Notify(ctx, fmt.Sprintf("🔨 Working on: %s", taskSummary))

		memCtx := buildMemoryContext(ctx, memory)

		task := coder.Task{
			ID:         uuid.New().String()[:8],
			Prompt:     params.Task,
			Complexity: complexity,
			Context:    memCtx,
			GitRepo:    params.GitRepo,
		}

		// use streaming for real-time progress
		onProgress := func(event coder.StreamEvent) {
			switch event.Type {
			case "thinking":
				registry.Notify(ctx, "💭 Thinking...")
			case "tool_use":
				registry.Notify(ctx, fmt.Sprintf("🔧 Using: %s", event.Tool))
			}
		}

		result, err := bridge.ExecuteWithProgress(ctx, task, onProgress)
		if err != nil {
			registry.Notify(ctx, fmt.Sprintf("❌ Code task failed: %v", err))
			return "", err
		}

		// notify completion
		registry.Notify(ctx, fmt.Sprintf("✅ Code complete: %d files created", len(result.Files)))

		return formatResult(result), nil
	})

	// cleanup workspaces tool
	cleanupTool := llm.Tool{
		Name:        "cleanup_workspaces",
		Description: "Remove old code workspaces to free up disk space. Removes workspaces older than the specified hours (default: 24).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"max_age_hours": map[string]any{
					"type":        "integer",
					"description": "Remove workspaces older than this many hours (default: 24)",
				},
			},
		},
	}

	registry.Register(cleanupTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			MaxAgeHours int `json:"max_age_hours"`
		}
		json.Unmarshal([]byte(args), &params)

		if params.MaxAgeHours <= 0 {
			params.MaxAgeHours = 24
		}

		maxAge := time.Duration(params.MaxAgeHours) * time.Hour
		count, err := bridge.CleanupWorkspaces(maxAge)
		if err != nil {
			return "", err
		}

		if count == 0 {
			return "No old workspaces to clean up", nil
		}

		return fmt.Sprintf("Cleaned up %d workspaces older than %d hours", count, params.MaxAgeHours), nil
	})
}

func buildMemoryContext(ctx context.Context, memory *sheldonmem.Store) *coder.MemoryContext {
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

	if result.WorkspacePath != "" {
		fmt.Fprintf(&sb, "Workspace: %s\n\n", result.WorkspacePath)
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
