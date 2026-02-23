package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bowerhall/sheldon/internal/config"
	"github.com/bowerhall/sheldon/internal/llm"
)

func RegisterModelTools(registry *Registry, rc *config.RuntimeConfig, mr *config.ModelRegistry) {
	registerListProviders(registry, mr)
	registerListModels(registry, mr)
	registerSwitchModel(registry, rc, mr)
	registerPullModel(registry, mr)
}

func registerListProviders(registry *Registry, mr *config.ModelRegistry) {
	tool := llm.Tool{
		Name:        "list_providers",
		Description: "List available LLM providers and their status (configured, available).",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		statuses := mr.ProviderStatus(ctx)

		var sb strings.Builder
		sb.WriteString("available providers:\n\n")

		for _, s := range statuses {
			configured := providerConfigured(s.ID)
			status := "not configured"
			if configured && s.Available {
				status = "ready"
			} else if configured && !s.Available {
				status = "configured but unavailable"
			}

			if s.EnvKey != "" {
				sb.WriteString(fmt.Sprintf("  %s (%s) [%s] - %s\n", s.ID, s.Name, status, s.EnvKey))
			} else {
				sb.WriteString(fmt.Sprintf("  %s (%s) [%s]\n", s.ID, s.Name, status))
			}
		}

		return sb.String(), nil
	})
}

func registerListModels(registry *Registry, mr *config.ModelRegistry) {
	tool := llm.Tool{
		Name:        "list_models",
		Description: "List available models. Use provider filter to see models for a specific provider.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"provider": map[string]any{
					"type":        "string",
					"description": "Filter by provider (kimi, claude, openai, ollama). Leave empty for all models.",
					"enum":        []string{"kimi", "claude", "openai", "ollama"},
				},
			},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Provider string `json:"provider"`
		}
		if args != "" && args != "{}" {
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
		}

		var models []config.ModelInfo
		var err error

		if params.Provider != "" {
			models, err = mr.ModelsByProvider(ctx, params.Provider)
		} else {
			models, err = mr.AllModels(ctx)
		}

		if err != nil {
			return "", err
		}

		if len(models) == 0 {
			if params.Provider != "" {
				return fmt.Sprintf("no models found for provider %q", params.Provider), nil
			}
			return "no models found", nil
		}

		var sb strings.Builder
		if params.Provider != "" {
			sb.WriteString(fmt.Sprintf("models for %s:\n\n", params.Provider))
		} else {
			sb.WriteString("available models:\n\n")
		}

		currentProvider := ""
		for _, m := range models {
			if params.Provider == "" && m.Provider != currentProvider {
				if currentProvider != "" {
					sb.WriteString("\n")
				}
				currentProvider = m.Provider
				sb.WriteString(fmt.Sprintf("[%s]\n", m.Provider))
			}

			location := "cloud"
			if m.Local {
				location = "local"
			}

			if len(m.Capabilities) > 0 {
				sb.WriteString(fmt.Sprintf("  %s (%s) [%s]\n", m.ID, m.Name, location))
			} else {
				sb.WriteString(fmt.Sprintf("  %s [%s]\n", m.ID, location))
			}
		}

		return sb.String(), nil
	})
}

func registerSwitchModel(registry *Registry, rc *config.RuntimeConfig, mr *config.ModelRegistry) {
	tool := llm.Tool{
		Name:        "switch_model",
		Description: "Switch the model used for a specific purpose (llm, extractor, embedder, coder). The change takes effect on the next message.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"purpose": map[string]any{
					"type":        "string",
					"description": "Which component to switch: llm (main chat), extractor (memory extraction), embedder (embeddings), coder (code generation)",
					"enum":        []string{"llm", "extractor", "embedder", "coder"},
				},
				"provider": map[string]any{
					"type":        "string",
					"description": "Provider to use (kimi, claude, openai, ollama). If omitted, will be inferred from model name.",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "Model ID to use (e.g., kimi-k2.5:cloud, claude-sonnet-4-20250514, gpt-4o)",
				},
			},
			"required": []string{"purpose", "model"},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Purpose  string `json:"purpose"`
			Provider string `json:"provider"`
			Model    string `json:"model"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		// infer provider from model if not specified
		provider := params.Provider
		if provider == "" {
			provider = inferProvider(params.Model, mr)
			if provider == "" {
				return "", fmt.Errorf("could not infer provider for model %q, please specify provider explicitly", params.Model)
			}
		}

		// validate provider is configured
		if !providerConfigured(provider) {
			envKey := envKeyForProvider(provider)
			return "", fmt.Errorf("cannot switch to %s: %s not configured", provider, envKey)
		}

		// validate purpose
		var providerKey, modelKey string
		switch params.Purpose {
		case "llm":
			providerKey = "llm_provider"
			modelKey = "llm_model"
		case "extractor":
			providerKey = "extractor_provider"
			modelKey = "extractor_model"
		case "embedder":
			providerKey = "embedder_provider"
			modelKey = "embedder_model"
		case "coder":
			providerKey = "coder_provider"
			modelKey = "coder_model"
		default:
			return "", fmt.Errorf("invalid purpose %q, must be one of: llm, extractor, embedder, coder", params.Purpose)
		}

		// set both provider and model
		oldProvider := rc.Get(providerKey)
		oldModel := rc.Get(modelKey)

		if err := rc.Set(providerKey, provider); err != nil {
			return "", fmt.Errorf("failed to set provider: %w", err)
		}
		if err := rc.Set(modelKey, params.Model); err != nil {
			return "", fmt.Errorf("failed to set model: %w", err)
		}

		registry.Notify(ctx, fmt.Sprintf("model switched: %s -> %s/%s", params.Purpose, provider, params.Model))

		return fmt.Sprintf("switched %s:\n  provider: %s -> %s\n  model: %s -> %s\n\nchange takes effect on next message",
			params.Purpose, oldProvider, provider, oldModel, params.Model), nil
	})
}

func inferProvider(model string, mr *config.ModelRegistry) string {
	// check cloud models first
	for _, m := range mr.CloudModels() {
		if m.ID == model {
			return m.Provider
		}
	}

	// check by prefix patterns
	switch {
	case strings.HasPrefix(model, "kimi-"):
		return "kimi"
	case strings.HasPrefix(model, "claude-"):
		return "claude"
	case strings.HasPrefix(model, "gpt-"):
		return "openai"
	case strings.Contains(model, ":"):
		return "ollama"
	}

	return ""
}

func envKeyForProvider(provider string) string {
	switch provider {
	case "claude":
		return "ANTHROPIC_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	case "kimi":
		return "KIMI_API_KEY"
	case "ollama":
		return ""
	default:
		// convention: {PROVIDER}_API_KEY
		return strings.ToUpper(provider) + "_API_KEY"
	}
}

func providerConfigured(provider string) bool {
	if provider == "ollama" {
		return true
	}
	envKey := envKeyForProvider(provider)
	if envKey == "" {
		return false
	}
	return os.Getenv(envKey) != ""
}

func registerPullModel(registry *Registry, mr *config.ModelRegistry) {
	tool := llm.Tool{
		Name:        "pull_model",
		Description: "Download a model from ollama. Use this to download new local models.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"model": map[string]any{
					"type":        "string",
					"description": "Model name to pull (e.g., llama3.2, qwen2.5:3b, nomic-embed-text)",
				},
			},
			"required": []string{"model"},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		registry.Notify(ctx, fmt.Sprintf("pulling model %s...", params.Model))

		if err := mr.PullModel(ctx, params.Model); err != nil {
			return "", fmt.Errorf("failed to pull model: %w", err)
		}

		registry.Notify(ctx, fmt.Sprintf("model %s pulled successfully", params.Model))

		return fmt.Sprintf("successfully pulled %s\n\nuse switch_model to activate it", params.Model), nil
	})
}
