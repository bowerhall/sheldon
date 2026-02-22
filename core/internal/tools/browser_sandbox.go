package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bowerhall/sheldon/internal/browser"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/logger"
)

// RegisterBrowserSandboxTools registers isolated browser automation tools
func RegisterBrowserSandboxTools(registry *Registry, runner *browser.Runner) {
	// browse tool - open URL and get snapshot
	browseTool := llm.Tool{
		Name:        "browse",
		Description: "Open a URL in a headless browser and get a snapshot of the page structure. Returns an accessibility tree with element references (@e1, @e2, etc.) that can be used with browse_click and browse_fill. Use this for JavaScript-heavy sites that require rendering.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The URL to open (must start with http:// or https://)",
				},
			},
			"required": []string{"url"},
		},
	}

	registry.Register(browseTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}

		logger.Debug("browse tool", "url", params.URL)

		result, err := runner.Browse(ctx, params.URL)
		if err != nil {
			return "", err
		}

		// truncate if too long
		if len(result) > 15000 {
			result = result[:15000] + "\n\n[Content truncated...]"
		}

		return result, nil
	})

	// browse_click tool - click an element
	clickTool := llm.Tool{
		Name:        "browse_click",
		Description: "Click an element on the page by its reference (e.g., @e1, @e2). Use the browse tool first to get element references from the page snapshot.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ref": map[string]any{
					"type":        "string",
					"description": "Element reference from snapshot (e.g., @e1, @e2)",
				},
			},
			"required": []string{"ref"},
		},
	}

	registry.Register(clickTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Ref string `json:"ref"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}

		// validate ref format
		if !strings.HasPrefix(params.Ref, "@e") {
			return "", fmt.Errorf("invalid ref format: must be @eN (e.g., @e1)")
		}

		logger.Debug("browse_click tool", "ref", params.Ref)

		result, err := runner.Click(ctx, params.Ref)
		if err != nil {
			return "", err
		}

		if len(result) > 15000 {
			result = result[:15000] + "\n\n[Content truncated...]"
		}

		return result, nil
	})

	// browse_fill tool - fill a form field
	fillTool := llm.Tool{
		Name:        "browse_fill",
		Description: "Fill a form field with text. Use the browse tool first to get element references from the page snapshot.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ref": map[string]any{
					"type":        "string",
					"description": "Element reference from snapshot (e.g., @e1, @e2)",
				},
				"value": map[string]any{
					"type":        "string",
					"description": "Text to fill into the field",
				},
			},
			"required": []string{"ref", "value"},
		},
	}

	registry.Register(fillTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Ref   string `json:"ref"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}

		// validate ref format
		if !strings.HasPrefix(params.Ref, "@e") {
			return "", fmt.Errorf("invalid ref format: must be @eN (e.g., @e1)")
		}

		logger.Debug("browse_fill tool", "ref", params.Ref, "value_len", len(params.Value))

		result, err := runner.Fill(ctx, params.Ref, params.Value)
		if err != nil {
			return "", err
		}

		if len(result) > 15000 {
			result = result[:15000] + "\n\n[Content truncated...]"
		}

		return result, nil
	})

	// browse_commands tool - run a sequence of browser commands
	commandsTool := llm.Tool{
		Name:        "browse_commands",
		Description: "Run a sequence of browser commands. Useful for complex interactions. Available commands: open, snapshot, click, fill, type, press, hover, scroll, wait, get, find, close.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"commands": map[string]any{
					"type":        "array",
					"description": "List of agent-browser commands to execute in order",
					"items": map[string]any{
						"type": "string",
					},
				},
			},
			"required": []string{"commands"},
		},
	}

	registry.Register(commandsTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Commands []string `json:"commands"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}

		if len(params.Commands) == 0 {
			return "", fmt.Errorf("no commands provided")
		}

		if len(params.Commands) > 20 {
			return "", fmt.Errorf("too many commands (max 20)")
		}

		logger.Debug("browse_commands tool", "count", len(params.Commands))

		result, err := runner.Run(ctx, params.Commands)
		if err != nil {
			return "", err
		}

		if len(result) > 15000 {
			result = result[:15000] + "\n\n[Content truncated...]"
		}

		return result, nil
	})
}
