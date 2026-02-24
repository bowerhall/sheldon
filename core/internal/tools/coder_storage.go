package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bowerhall/sheldon/internal/coder"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/storage"
)

func RegisterCoderStorageTools(registry *Registry, bridge *coder.Bridge, client *storage.Client) {
	listTool := llm.Tool{
		Name:        "list_storage_images",
		Description: "List images available in storage that can be used in code projects. Returns paths that can be used with fetch_to_workspace.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"space": map[string]any{
					"type":        "string",
					"enum":        []string{"user", "agent"},
					"description": "Storage space: 'user' for user files, 'agent' for agent files",
				},
				"prefix": map[string]any{
					"type":        "string",
					"description": "Optional path prefix to filter (e.g., 'images/' to list only images folder)",
				},
			},
			"required": []string{"space"},
		},
	}

	registry.Register(listTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space  string `json:"space"`
			Prefix string `json:"prefix"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		bucket := client.UserBucket()
		if params.Space == "agent" {
			bucket = client.AgentBucket()
		}

		files, err := client.List(ctx, bucket, params.Prefix)
		if err != nil {
			return "", err
		}

		var images []storage.FileInfo
		for _, f := range files {
			if isImageFile(f.Name) {
				images = append(images, f)
			}
		}

		if len(images) == 0 {
			return fmt.Sprintf("no images found in %s/%s", params.Space, params.Prefix), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Images in %s/%s:\n", params.Space, params.Prefix))
		for _, f := range images {
			sb.WriteString(fmt.Sprintf("  - %s (%d bytes)\n", f.Name, f.Size))
		}
		sb.WriteString("\nUse fetch_to_workspace to download these to a coder workspace.")

		return sb.String(), nil
	})

	fetchTool := llm.Tool{
		Name:        "fetch_to_workspace",
		Description: "Download a file from storage to a coder workspace. Use this to make images or other assets available in a code project. The file will be placed in the workspace's assets directory.",
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
					"description": "Path to file in storage",
				},
				"workspace_id": map[string]any{
					"type":        "string",
					"description": "Task ID of the coder workspace (from write_code result)",
				},
				"dest_path": map[string]any{
					"type":        "string",
					"description": "Destination path within workspace (e.g., 'assets/logo.png', 'public/images/hero.jpg'). Directories will be created if needed.",
				},
			},
			"required": []string{"space", "path", "workspace_id", "dest_path"},
		},
	}

	registry.Register(fetchTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space       string `json:"space"`
			Path        string `json:"path"`
			WorkspaceID string `json:"workspace_id"`
			DestPath    string `json:"dest_path"`
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
			return "", fmt.Errorf("download from storage: %w", err)
		}

		workspacePath, err := bridge.GetLocalWorkspacePath(ctx, params.WorkspaceID)
		if err != nil {
			return "", fmt.Errorf("get workspace path: %w", err)
		}

		fullPath := filepath.Join(workspacePath, params.DestPath)

		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return "", fmt.Errorf("create directories: %w", err)
		}

		if err := os.WriteFile(fullPath, data, 0644); err != nil {
			return "", fmt.Errorf("write file: %w", err)
		}

		return fmt.Sprintf("downloaded %s/%s to workspace %s at %s (%d bytes)",
			params.Space, params.Path, params.WorkspaceID, params.DestPath, len(data)), nil
	})
}

func isImageFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".bmp":
		return true
	default:
		return false
	}
}
