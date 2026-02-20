package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/storage"
)

// RegisterStorageTools registers MinIO file storage tools
func RegisterStorageTools(registry *Registry, client *storage.Client) {
	// upload file tool
	uploadTool := llm.Tool{
		Name:        "upload_file",
		Description: "Upload a file to storage. Use 'user' space for user files, 'agent' space for agent's own files (notes, exports, artifacts).",
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
					"description": "File path including name (e.g., 'notes/todo.txt', 'exports/data.json')",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "File content as text. For binary files, use base64 encoding and set is_base64=true",
				},
				"is_base64": map[string]any{
					"type":        "boolean",
					"description": "Set to true if content is base64 encoded (for binary files)",
				},
			},
			"required": []string{"space", "path", "content"},
		},
	}

	registry.Register(uploadTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space    string `json:"space"`
			Path     string `json:"path"`
			Content  string `json:"content"`
			IsBase64 bool   `json:"is_base64"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		bucket := client.UserBucket()
		if params.Space == "agent" {
			bucket = client.AgentBucket()
		}

		var data []byte
		var err error
		if params.IsBase64 {
			data, err = base64.StdEncoding.DecodeString(params.Content)
			if err != nil {
				return "", fmt.Errorf("invalid base64: %w", err)
			}
		} else {
			data = []byte(params.Content)
		}

		contentType := guessContentType(params.Path)
		if err := client.Upload(ctx, bucket, params.Path, data, contentType); err != nil {
			return "", err
		}

		return fmt.Sprintf("uploaded %s to %s (%d bytes)", params.Path, params.Space, len(data)), nil
	})

	// download file tool
	downloadTool := llm.Tool{
		Name:        "download_file",
		Description: "Download a file from storage. Returns file content as text (or base64 for binary files).",
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
					"description": "File path to download",
				},
			},
			"required": []string{"space", "path"},
		},
	}

	registry.Register(downloadTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space string `json:"space"`
			Path  string `json:"path"`
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
			return "", err
		}

		// check if binary
		if isBinary(data) {
			return fmt.Sprintf("base64:%s", base64.StdEncoding.EncodeToString(data)), nil
		}

		return string(data), nil
	})

	// list files tool
	listTool := llm.Tool{
		Name:        "list_files",
		Description: "List files in storage. Returns file names, sizes, and modification times.",
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
					"description": "Optional path prefix to filter files (e.g., 'notes/' to list only notes folder)",
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

		if len(files) == 0 {
			return fmt.Sprintf("no files in %s/%s", params.Space, params.Prefix), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("files in %s/%s:\n", params.Space, params.Prefix))
		for _, f := range files {
			if f.IsDir {
				sb.WriteString(fmt.Sprintf("  📁 %s\n", f.Name))
			} else {
				sb.WriteString(fmt.Sprintf("  📄 %s (%d bytes, %s)\n", f.Name, f.Size, f.ModTime))
			}
		}

		return sb.String(), nil
	})

	// delete file tool
	deleteTool := llm.Tool{
		Name:        "delete_file",
		Description: "Delete a file from storage.",
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
					"description": "File path to delete",
				},
			},
			"required": []string{"space", "path"},
		},
	}

	registry.Register(deleteTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space string `json:"space"`
			Path  string `json:"path"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		bucket := client.UserBucket()
		if params.Space == "agent" {
			bucket = client.AgentBucket()
		}

		if err := client.Delete(ctx, bucket, params.Path); err != nil {
			return "", err
		}

		return fmt.Sprintf("deleted %s from %s", params.Path, params.Space), nil
	})
}

func guessContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

func isBinary(data []byte) bool {
	// check for null bytes or high proportion of non-printable chars
	for _, b := range data[:min(len(data), 512)] {
		if b == 0 {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
