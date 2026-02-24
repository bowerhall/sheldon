package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/storage"
)

type PhotoSender interface {
	SendPhoto(chatID int64, data []byte, caption string) error
}

func RegisterImageTools(registry *Registry, sender PhotoSender, client *storage.Client) {
	sendTool := llm.Tool{
		Name:        "send_image",
		Description: "Send an image from storage to the user. Use this to share images, photos, charts, or other visual content that has been saved to storage.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"space": map[string]any{
					"type":        "string",
					"enum":        []string{"user", "agent"},
					"description": "Storage space: 'user' for user files, 'agent' for agent files",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Path to image in storage",
				},
				"caption": map[string]any{
					"type":        "string",
					"description": "Optional caption for the image",
				},
			},
			"required": []string{"space", "path"},
		},
	}

	registry.Register(sendTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space   string `json:"space"`
			Path    string `json:"path"`
			Caption string `json:"caption"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		bucket := client.UserBucket()
		if params.Space == "agent" {
			bucket = client.AgentBucket()
		}

		data, err := client.Download(ctx, bucket, params.Path)
		if err != nil {
			return "", fmt.Errorf("download image: %w", err)
		}

		chatID := ChatIDFromContext(ctx)
		if chatID == 0 {
			return "", fmt.Errorf("no chat ID in context")
		}

		if err := sender.SendPhoto(chatID, data, params.Caption); err != nil {
			return "", fmt.Errorf("send photo: %w", err)
		}

		return fmt.Sprintf("sent image %s/%s to user", params.Space, params.Path), nil
	})
}
