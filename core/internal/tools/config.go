package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/bowerhall/sheldon/internal/config"
	"github.com/bowerhall/sheldon/internal/llm"
)

// RegisterConfigTools registers runtime config management tools
func RegisterConfigTools(registry *Registry, rc *config.RuntimeConfig) {
	// get config tool
	getTool := llm.Tool{
		Name:        "get_config",
		Description: "Get current runtime configuration values (models, providers). Shows both current values and any overrides.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	registry.Register(getTool, func(ctx context.Context, args string) (string, error) {
		all := rc.All()
		overrides := rc.Overrides()

		var sb strings.Builder
		sb.WriteString("current configuration:\n\n")

		// sort keys for consistent output
		keys := make([]string, 0, len(all))
		for k := range all {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := all[k]
			if v == "" {
				v = "(not set)"
			}
			if _, isOverride := overrides[k]; isOverride {
				sb.WriteString(fmt.Sprintf("  %s: %s (override)\n", k, v))
			} else {
				sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
			}
		}

		sb.WriteString("\nallowed keys:\n")
		for k, desc := range config.AllowedKeys {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, desc))
		}

		return sb.String(), nil
	})

	// set config tool
	setTool := llm.Tool{
		Name:        "set_config",
		Description: "Change a runtime configuration value. Only non-secret values (models, providers) can be changed. Changes persist across restarts.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"enum":        getAllowedKeys(),
					"description": "Configuration key to change",
				},
				"value": map[string]any{
					"type":        "string",
					"description": "New value for the configuration",
				},
			},
			"required": []string{"key", "value"},
		},
	}

	registry.Register(setTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		oldValue := rc.Get(params.Key)
		if err := rc.Set(params.Key, params.Value); err != nil {
			return "", err
		}

		// notify about the change
		registry.Notify(ctx, fmt.Sprintf("⚙️ Config changed: %s", params.Key))

		return fmt.Sprintf("changed %s: %q → %q\n\nnote: LLM changes take effect on next message (current conversation continues with old model)", params.Key, oldValue, params.Value), nil
	})

	// reset config tool
	resetTool := llm.Tool{
		Name:        "reset_config",
		Description: "Reset a runtime configuration value to its default (from environment variables). Use key='all' to reset everything.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "Configuration key to reset, or 'all' to reset everything",
				},
			},
			"required": []string{"key"},
		},
	}

	registry.Register(resetTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if params.Key == "all" {
			if err := rc.ResetAll(); err != nil {
				return "", err
			}
			return "reset all runtime config overrides to defaults", nil
		}

		if err := rc.Reset(params.Key); err != nil {
			return "", err
		}

		newValue := rc.Get(params.Key)
		return fmt.Sprintf("reset %s to default: %q", params.Key, newValue), nil
	})
}

func getAllowedKeys() []string {
	keys := make([]string, 0, len(config.AllowedKeys))
	for k := range config.AllowedKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
