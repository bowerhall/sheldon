package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

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

	saveTool := llm.Tool{
		Name:        "save_image",
		Description: "Save an image from the current message to storage. Use this when the user sends a photo and asks you to save it. Only works when there's an image in the current message.",
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
					"description": "Path to save the image (e.g., 'photos/vacation.jpg'). If not provided, generates a timestamped name.",
				},
				"index": map[string]any{
					"type":        "integer",
					"description": "Which image to save if multiple were sent (0-indexed, default 0)",
				},
			},
			"required": []string{"space"},
		},
	}

	registry.Register(saveTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space string `json:"space"`
			Path  string `json:"path"`
			Index int    `json:"index"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		images := ImagesFromContext(ctx)
		if len(images) == 0 {
			return "", fmt.Errorf("no images in current message")
		}

		if params.Index >= len(images) {
			return "", fmt.Errorf("image index %d out of range (have %d images)", params.Index, len(images))
		}

		img := images[params.Index]

		bucket := client.UserBucket()
		if params.Space == "agent" {
			bucket = client.AgentBucket()
		}

		path := params.Path
		if path == "" {
			ext := extensionForMediaType(img.MediaType)
			path = fmt.Sprintf("images/image_%s%s", time.Now().Format("2006-01-02_15-04-05"), ext)
		}

		if err := client.Upload(ctx, bucket, path, img.Data, img.MediaType); err != nil {
			return "", fmt.Errorf("upload image: %w", err)
		}

		return fmt.Sprintf("saved image to %s/%s (%d bytes)", params.Space, path, len(img.Data)), nil
	})
}

func extensionForMediaType(mediaType string) string {
	switch mediaType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		ext := filepath.Ext(http.DetectContentType([]byte{}))
		if ext != "" {
			return ext
		}
		return ".jpg"
	}
}
