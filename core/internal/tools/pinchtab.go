package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/pinchtab"
)

func RegisterPinchtabTools(registry *Registry, client *pinchtab.Client) {
	if client == nil {
		return
	}

	registerBrowseSession(registry, client)
	registerSessionAction(registry, client)
}

func registerBrowseSession(registry *Registry, client *pinchtab.Client) {
	tool := llm.Tool{
		Name:        "browse_session",
		Description: "Browse a URL with persistent session (cookies, login state preserved). Use for authenticated sites like Gmail, GitHub, etc. REQUIRES USER APPROVAL.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "URL to browse",
				},
				"profile": map[string]any{
					"type":        "string",
					"description": "Browser profile name for session persistence (default: 'default')",
				},
			},
			"required": []string{"url"},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			URL     string `json:"url"`
			Profile string `json:"profile"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if params.Profile == "" {
			params.Profile = "default"
		}

		if !strings.HasPrefix(params.URL, "http://") && !strings.HasPrefix(params.URL, "https://") {
			return "", fmt.Errorf("URL must start with http:// or https://")
		}

		instance, err := client.CreateInstance(ctx, params.Profile)
		if err != nil {
			return "", fmt.Errorf("failed to create browser instance: %w", err)
		}

		if err := client.Navigate(ctx, instance.ID, params.URL); err != nil {
			client.CloseInstance(ctx, instance.ID)
			return "", fmt.Errorf("failed to navigate: %w", err)
		}

		text, err := client.Text(ctx, instance.ID)
		if err != nil {
			client.CloseInstance(ctx, instance.ID)
			return "", fmt.Errorf("failed to get page text: %w", err)
		}

		if len(text) > 4000 {
			text = text[:4000] + "\n... (truncated)"
		}

		return fmt.Sprintf("Instance: %s\nProfile: %s\nURL: %s\n\nContent:\n%s",
			instance.ID, params.Profile, params.URL, text), nil
	})
}

func registerSessionAction(registry *Registry, client *pinchtab.Client) {
	tool := llm.Tool{
		Name:        "session_action",
		Description: "Perform an action on an existing browser session (click, fill, press). Use after browse_session.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"instance_id": map[string]any{
					"type":        "string",
					"description": "Browser instance ID from browse_session",
				},
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"click", "fill", "press", "snapshot", "text", "close"},
					"description": "Action to perform",
				},
				"ref": map[string]any{
					"type":        "string",
					"description": "Element reference (for click, fill)",
				},
				"value": map[string]any{
					"type":        "string",
					"description": "Value to fill or key to press",
				},
			},
			"required": []string{"instance_id", "action"},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			InstanceID string `json:"instance_id"`
			Action     string `json:"action"`
			Ref        string `json:"ref"`
			Value      string `json:"value"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		switch params.Action {
		case "click":
			if params.Ref == "" {
				return "", fmt.Errorf("ref required for click")
			}
			if err := client.Click(ctx, params.InstanceID, params.Ref); err != nil {
				return "", err
			}
			return "clicked " + params.Ref, nil

		case "fill":
			if params.Ref == "" || params.Value == "" {
				return "", fmt.Errorf("ref and value required for fill")
			}
			if err := client.Fill(ctx, params.InstanceID, params.Ref, params.Value); err != nil {
				return "", err
			}
			return "filled " + params.Ref, nil

		case "press":
			if params.Value == "" {
				return "", fmt.Errorf("value (key) required for press")
			}
			if err := client.Press(ctx, params.InstanceID, params.Value); err != nil {
				return "", err
			}
			return "pressed " + params.Value, nil

		case "snapshot":
			snapshot, err := client.Snapshot(ctx, params.InstanceID)
			if err != nil {
				return "", err
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("URL: %s\nTitle: %s\n\nElements:\n", snapshot.URL, snapshot.Title))
			for _, el := range snapshot.Elements {
				if el.Name != "" || el.Text != "" {
					sb.WriteString(fmt.Sprintf("  [%s] %s: %s %s\n", el.Ref, el.Role, el.Name, el.Text))
				}
			}
			return sb.String(), nil

		case "text":
			text, err := client.Text(ctx, params.InstanceID)
			if err != nil {
				return "", err
			}
			if len(text) > 4000 {
				text = text[:4000] + "\n... (truncated)"
			}
			return text, nil

		case "close":
			if err := client.CloseInstance(ctx, params.InstanceID); err != nil {
				return "", err
			}
			return "session closed", nil

		default:
			return "", fmt.Errorf("unknown action: %s", params.Action)
		}
	})
}
