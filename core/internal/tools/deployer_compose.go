package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bowerhall/sheldon/internal/deployer"
	"github.com/bowerhall/sheldon/internal/llm"
)

type ComposeDeployArgs struct {
	AppDir string `json:"app_dir"`
	Name   string `json:"name"`
	Domain string `json:"domain,omitempty"`
}

type BuildArgs struct {
	ContextDir string `json:"context_dir"`
	ImageName  string `json:"image_name"`
	ImageTag   string `json:"image_tag,omitempty"`
}

type ComposeServiceArgs struct {
	Name string `json:"name"`
}

func RegisterComposeDeployerTools(registry *Registry, builder *deployer.Builder, deploy *deployer.ComposeDeployer, domain string) {
	deployTool := llm.Tool{
		Name:        "deploy_app",
		Description: "Deploy an app using Docker Compose. The app directory should contain a Dockerfile. Sheldon will build the image and add it to the apps.yml file with Traefik routing.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"app_dir": map[string]any{
					"type":        "string",
					"description": "Directory containing the app code and Dockerfile",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Name for the app (used for routing: name.yourdomain.com)",
				},
			},
			"required": []string{"app_dir", "name"},
		},
	}

	registry.Register(deployTool, func(ctx context.Context, args string) (string, error) {
		var params ComposeDeployArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		registry.Notify(ctx, fmt.Sprintf("🚀 Deploying %s...", params.Name))

		result, err := deploy.Deploy(ctx, params.AppDir, params.Name, domain)
		if err != nil {
			registry.Notify(ctx, fmt.Sprintf("❌ Deploy failed: %v", err))
			return "", err
		}

		url := fmt.Sprintf("%s.%s", params.Name, domain)
		registry.Notify(ctx, fmt.Sprintf("✅ Deployed: %s → http://%s", params.Name, url))

		return fmt.Sprintf("App deployed: %s\nURL: http://%s\nStatus: %s",
			strings.Join(result.Resources, ", "), url, result.Status), nil
	})

	removeTool := llm.Tool{
		Name:        "remove_app",
		Description: "Stop and remove a deployed app from Docker Compose.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the app to remove",
				},
			},
			"required": []string{"name"},
		},
	}

	registry.Register(removeTool, func(ctx context.Context, args string) (string, error) {
		var params ComposeServiceArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if err := deploy.Remove(ctx, params.Name); err != nil {
			return "", err
		}

		return fmt.Sprintf("App %s removed", params.Name), nil
	})

	listTool := llm.Tool{
		Name:        "list_apps",
		Description: "List all deployed apps managed by Sheldon.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	registry.Register(listTool, func(ctx context.Context, args string) (string, error) {
		apps, err := deploy.List(ctx)
		if err != nil {
			return "", err
		}

		if len(apps) == 0 {
			return "No apps deployed yet.", nil
		}

		return fmt.Sprintf("Deployed apps:\n- %s", strings.Join(apps, "\n- ")), nil
	})

	statusTool := llm.Tool{
		Name:        "app_status",
		Description: "Check the status of a deployed app.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the app to check",
				},
			},
			"required": []string{"name"},
		},
	}

	registry.Register(statusTool, func(ctx context.Context, args string) (string, error) {
		var params ComposeServiceArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		status, err := deploy.Status(ctx, params.Name)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("App %s: %s", params.Name, status), nil
	})

	logsTool := llm.Tool{
		Name:        "app_logs",
		Description: "Get recent logs from a deployed app.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the app",
				},
			},
			"required": []string{"name"},
		},
	}

	registry.Register(logsTool, func(ctx context.Context, args string) (string, error) {
		var params ComposeServiceArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		logs, err := deploy.Logs(ctx, params.Name, 50)
		if err != nil {
			return "", err
		}

		return logs, nil
	})

	buildTool := llm.Tool{
		Name:        "build_image",
		Description: "Build a Docker image from a directory containing a Dockerfile. Use this before deploy_app if you want to pre-build the image.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"context_dir": map[string]any{
					"type":        "string",
					"description": "Directory containing the Dockerfile and source code",
				},
				"image_name": map[string]any{
					"type":        "string",
					"description": "Name for the image (e.g., 'myapp', 'weather-bot')",
				},
				"image_tag": map[string]any{
					"type":        "string",
					"description": "Tag for the image (default: 'latest')",
				},
			},
			"required": []string{"context_dir", "image_name"},
		},
	}

	registry.Register(buildTool, func(ctx context.Context, args string) (string, error) {
		var params BuildArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		tag := params.ImageTag
		if tag == "" {
			tag = "latest"
		}

		registry.Notify(ctx, fmt.Sprintf("🐳 Building image: %s:%s", params.ImageName, tag))

		result, err := builder.Build(ctx, params.ContextDir, params.ImageName, tag)
		if err != nil {
			registry.Notify(ctx, fmt.Sprintf("❌ Build failed: %v", err))
			return "", err
		}

		registry.Notify(ctx, fmt.Sprintf("✅ Image built: %s:%s (%s)", result.ImageName, result.ImageTag, result.Duration))

		return fmt.Sprintf("Image built: %s:%s (%d bytes, %s)",
			result.ImageName, result.ImageTag, result.Size, result.Duration), nil
	})

	cleanupTool := llm.Tool{
		Name:        "cleanup_images",
		Description: "Remove unused container images to free up disk space.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	registry.Register(cleanupTool, func(ctx context.Context, args string) (string, error) {
		count, err := builder.Cleanup(ctx, 0)
		if err != nil {
			return "", err
		}

		if count == 0 {
			return "No unused images to clean up", nil
		}

		return fmt.Sprintf("Cleaned up %d unused images", count), nil
	})
}
