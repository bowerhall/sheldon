package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/storage"
)

type MediaSender interface {
	SendPhoto(chatID int64, data []byte, caption string) error
	SendVideo(chatID int64, data []byte, caption string) error
	SendDocument(chatID int64, data []byte, filename, caption string) error
}

func RegisterMediaTools(registry *Registry, sender MediaSender, client *storage.Client) {
	// send_image - send image from storage to user
	sendImageTool := llm.Tool{
		Name:        "send_image",
		Description: "Send an image from storage to the user.",
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

	registry.Register(sendImageTool, func(ctx context.Context, args string) (string, error) {
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

	// send_video - send video from storage to user
	sendVideoTool := llm.Tool{
		Name:        "send_video",
		Description: "Send a video from storage to the user.",
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
					"description": "Path to video in storage",
				},
				"caption": map[string]any{
					"type":        "string",
					"description": "Optional caption for the video",
				},
			},
			"required": []string{"space", "path"},
		},
	}

	registry.Register(sendVideoTool, func(ctx context.Context, args string) (string, error) {
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
			return "", fmt.Errorf("download video: %w", err)
		}

		chatID := ChatIDFromContext(ctx)
		if chatID == 0 {
			return "", fmt.Errorf("no chat ID in context")
		}

		if err := sender.SendVideo(chatID, data, params.Caption); err != nil {
			return "", fmt.Errorf("send video: %w", err)
		}

		return fmt.Sprintf("sent video %s/%s to user", params.Space, params.Path), nil
	})

	// save_media - save image or video from current message to storage
	saveMediaTool := llm.Tool{
		Name:        "save_media",
		Description: "Save an image or video from the current message to storage. Use this when the user sends a photo/video and asks you to save it.",
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
					"description": "Path to save the media (e.g., 'photos/vacation.jpg', 'videos/clip.mp4'). If not provided, generates a timestamped name.",
				},
				"index": map[string]any{
					"type":        "integer",
					"description": "Which media item to save if multiple were sent (0-indexed, default 0)",
				},
			},
			"required": []string{"space"},
		},
	}

	registry.Register(saveMediaTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space string `json:"space"`
			Path  string `json:"path"`
			Index int    `json:"index"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		media := MediaFromContext(ctx)
		if len(media) == 0 {
			return "", fmt.Errorf("no media in current message")
		}

		if params.Index >= len(media) {
			return "", fmt.Errorf("media index %d out of range (have %d items)", params.Index, len(media))
		}

		item := media[params.Index]

		bucket := client.UserBucket()
		if params.Space == "agent" {
			bucket = client.AgentBucket()
		}

		path := params.Path
		if path == "" {
			ext := extensionForMimeType(item.MimeType)
			folder := "images"
			if item.Type == llm.MediaTypeVideo {
				folder = "videos"
			}
			path = fmt.Sprintf("%s/%s_%s%s", folder, item.Type, time.Now().Format("2006-01-02_15-04-05"), ext)
		}

		if err := client.Upload(ctx, bucket, path, item.Data, item.MimeType); err != nil {
			return "", fmt.Errorf("upload media: %w", err)
		}

		return fmt.Sprintf("saved %s to %s/%s (%d bytes)", item.Type, params.Space, path, len(item.Data)), nil
	})
}

func extensionForMimeType(mimeType string) string {
	switch mimeType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "video/x-msvideo":
		return ".avi"
	default:
		if strings.HasPrefix(mimeType, "video/") {
			return ".mp4"
		}
		return ".jpg"
	}
}
