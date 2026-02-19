package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kadet/kora/internal/deployer"
	"github.com/kadet/kora/internal/llm"
)

type BuildArgs struct {
	ContextDir string `json:"context_dir"`
	ImageName  string `json:"image_name"`
	ImageTag   string `json:"image_tag,omitempty"`
}

type DeployArgs struct {
	ManifestDir string `json:"manifest_dir"`
	Namespace   string `json:"namespace,omitempty"`
}

type DeploymentArgs struct {
	DeploymentName string `json:"deployment_name"`
	Namespace      string `json:"namespace,omitempty"`
}

func RegisterDeployerTools(registry *Registry, builder *deployer.Builder, deploy *deployer.Deployer) {
	buildTool := llm.Tool{
		Name:        "build_image",
		Description: "Build a Docker image from a directory containing a Dockerfile. The image is automatically imported into the k8s cluster. Use this after write_code has created a Dockerfile.",
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

		return fmt.Sprintf("Image built and imported: %s:%s (%d bytes, %s)",
			result.ImageName, result.ImageTag, result.Size, result.Duration), nil
	})

	deployTool := llm.Tool{
		Name:        "deploy",
		Description: "Deploy Kubernetes manifests to the cluster. Applies all YAML files in the specified directory. Use this after build_image to deploy the application.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"manifest_dir": map[string]any{
					"type":        "string",
					"description": "Directory containing Kubernetes YAML manifests",
				},
				"namespace": map[string]any{
					"type":        "string",
					"description": "Kubernetes namespace (default: 'kora-apps')",
				},
			},
			"required": []string{"manifest_dir"},
		},
	}

	registry.Register(deployTool, func(ctx context.Context, args string) (string, error) {
		var params DeployArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		d := deploy
		if params.Namespace != "" {
			d = deployer.NewDeployer(params.Namespace)
		}

		registry.Notify(ctx, fmt.Sprintf("🚀 Deploying to %s...", d.Namespace()))

		result, err := d.Deploy(ctx, params.ManifestDir)
		if err != nil {
			registry.Notify(ctx, fmt.Sprintf("❌ Deploy failed: %v", err))
			return "", err
		}

		registry.Notify(ctx, fmt.Sprintf("✅ Deployed: %s", strings.Join(result.Resources, ", ")))

		return fmt.Sprintf("Deployed to %s: %s\nStatus: %s",
			result.Namespace, strings.Join(result.Resources, ", "), result.Status), nil
	})

	rollbackTool := llm.Tool{
		Name:        "rollback",
		Description: "Rollback a deployment to its previous version. Use this if a deployment is failing or behaving incorrectly.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deployment_name": map[string]any{
					"type":        "string",
					"description": "Name of the deployment to rollback",
				},
				"namespace": map[string]any{
					"type":        "string",
					"description": "Kubernetes namespace (default: 'kora-apps')",
				},
			},
			"required": []string{"deployment_name"},
		},
	}

	registry.Register(rollbackTool, func(ctx context.Context, args string) (string, error) {
		var params DeploymentArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		d := deploy
		if params.Namespace != "" {
			d = deployer.NewDeployer(params.Namespace)
		}

		if err := d.Rollback(ctx, params.DeploymentName); err != nil {
			return "", err
		}

		return fmt.Sprintf("Rolled back deployment/%s in %s", params.DeploymentName, d.Namespace()), nil
	})

	statusTool := llm.Tool{
		Name:        "deployment_status",
		Description: "Check the rollout status of a deployment. Use this to verify a deployment completed successfully.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deployment_name": map[string]any{
					"type":        "string",
					"description": "Name of the deployment to check",
				},
				"namespace": map[string]any{
					"type":        "string",
					"description": "Kubernetes namespace (default: 'kora-apps')",
				},
			},
			"required": []string{"deployment_name"},
		},
	}

	registry.Register(statusTool, func(ctx context.Context, args string) (string, error) {
		var params DeploymentArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		d := deploy
		if params.Namespace != "" {
			d = deployer.NewDeployer(params.Namespace)
		}

		status, err := d.Status(ctx, params.DeploymentName)
		if err != nil {
			return status, err
		}

		return status, nil
	})

	cleanupTool := llm.Tool{
		Name:        "cleanup_images",
		Description: "Remove unused container images from the cluster to free up disk space. Run this periodically or when storage is low.",
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
