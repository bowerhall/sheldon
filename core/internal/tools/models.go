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
	registerCurrentModel(registry, rc)
	registerListProviders(registry, mr)
	registerListModels(registry, mr)
	registerSwitchModel(registry, rc, mr)
	registerPullModel(registry, mr)
	registerRemoveModel(registry, mr)
}

func registerCurrentModel(registry *Registry, rc *config.RuntimeConfig) {
	tool := llm.Tool{
		Name:        "current_model",
		Description: "Check which LLM model is currently active for chat and coding.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		llmProvider := rc.Get("llm_provider")
		llmModel := rc.Get("llm_model")
		coderProvider := rc.Get("coder_provider")
		coderModel := rc.Get("coder_model")

		var sb strings.Builder
		sb.WriteString("current models:\n\n")
		sb.WriteString(fmt.Sprintf("  chat (llm): %s/%s\n", llmProvider, llmModel))
		sb.WriteString(fmt.Sprintf("  coder: %s/%s\n", coderProvider, coderModel))

		return sb.String(), nil
	})
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
		Name: "switch_model",
		Description: `Switch the model used for a specific purpose. IMPORTANT: Before switching, you MUST:
1. Call list_providers to check which providers are configured (have API keys)
2. Call list_models with the provider filter to show available models
3. Ask the user to confirm which specific model they want
4. Only then call switch_model with the confirmed choice

Never assume which model the user wants. Always show options and get explicit confirmation first.

NOTE: Only 'llm' and 'coder' can be switched. Extractor and embedder are core infrastructure -
changing embedder would break vector compatibility with existing memories. If user asks to change
these, explain why it's not supported and suggest they modify the server config instead.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"purpose": map[string]any{
					"type":        "string",
					"description": "Which component to switch: llm (main chat) or coder (code generation). Extractor and embedder cannot be changed at runtime.",
					"enum":        []string{"llm", "coder"},
				},
				"provider": map[string]any{
					"type":        "string",
					"description": "Provider to use (kimi, claude, openai, ollama). If omitted, will be inferred from model name.",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "Model ID to use (e.g., kimi-k2-0711-preview, claude-sonnet-4-20250514, gpt-4o)",
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
			envKey := config.EnvKeyForProvider(provider)
			return "", fmt.Errorf("cannot switch to %s: %s not configured", provider, envKey)
		}

		// validate model has the right capability for the purpose
		requiredCap := purposeToCapability(params.Purpose)
		if requiredCap != "" && !modelHasCapability(params.Model, requiredCap, mr) {
			return "", fmt.Errorf("model %q does not support %s (required for %s purpose)", params.Model, requiredCap, params.Purpose)
		}

		// validate purpose
		var providerKey, modelKey string
		switch params.Purpose {
		case "llm":
			providerKey = "llm_provider"
			modelKey = "llm_model"
		case "coder":
			providerKey = "coder_provider"
			modelKey = "coder_model"
		case "extractor", "embedder":
			return "", fmt.Errorf("cannot switch %s at runtime - it's core infrastructure. Changing embedder would break vector compatibility with existing memories. Modify server environment config instead", params.Purpose)
		default:
			return "", fmt.Errorf("invalid purpose %q, must be one of: llm, coder", params.Purpose)
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

// purposeToCapability maps a switch purpose to the required model capability
func purposeToCapability(purpose string) string {
	switch purpose {
	case "llm":
		return "chat"
	case "coder":
		return "code"
	case "extractor", "embedder":
		return "" // no specific capability required
	default:
		return ""
	}
}

// modelHasCapability checks if a model has a specific capability
func modelHasCapability(modelID, capability string, mr *config.ModelRegistry) bool {
	for _, m := range mr.CloudModels() {
		if m.ID == modelID {
			for _, cap := range m.Capabilities {
				if cap == capability {
					return true
				}
			}
			// model found but doesn't have capability
			return false
		}
	}
	// model not in cloud list - allow it (could be ollama model)
	return true
}

func providerConfigured(provider string) bool {
	if provider == "ollama" {
		return true
	}
	envKey := config.EnvKeyForProvider(provider)
	if envKey == "" {
		return false
	}
	return os.Getenv(envKey) != ""
}

func registerPullModel(registry *Registry, mr *config.ModelRegistry) {
	tool := llm.Tool{
		Name: "pull_model",
		Description: `Download a model from ollama. IMPORTANT: Before pulling, you MUST:
1. Call system_status to check available disk space
2. Confirm the user explicitly wants to download this specific model
3. Warn if disk space is tight (less than 10GB for large models, less than 2GB for small models)
4. Explain the model size and that it will take time to download
5. Only proceed after getting explicit confirmation

Never pull models just because the user asked IF you can - only pull when they explicitly request it.
If disk space is critically low (<2GB available), refuse to pull and suggest removing unused models first.`,
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

func registerRemoveModel(registry *Registry, mr *config.ModelRegistry) {
	tool := llm.Tool{
		Name: "remove_model",
		Description: `Remove a downloaded ollama model to free up disk space. IMPORTANT: Before removing:
1. Confirm the user wants to delete this specific model
2. Warn that this will require re-downloading if needed later
3. Check that it's not currently in use (extractor, embedder, coder)

Never remove models without explicit confirmation.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"model": map[string]any{
					"type":        "string",
					"description": "Model name to remove (e.g., qwen2.5:0.5b, llama3.2)",
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

		// protect critical infrastructure models
		embedderModel := os.Getenv("EMBEDDER_MODEL")
		extractorModel := os.Getenv("EXTRACTOR_MODEL")

		if params.Model == embedderModel || strings.HasPrefix(embedderModel, params.Model) {
			return "", fmt.Errorf("cannot remove %s - it's the active embedder model. Removing it would break memory search", params.Model)
		}
		if params.Model == extractorModel || strings.HasPrefix(extractorModel, params.Model) {
			return "", fmt.Errorf("cannot remove %s - it's the active extractor model. Removing it would break memory extraction", params.Model)
		}

		registry.Notify(ctx, fmt.Sprintf("removing model %s...", params.Model))

		if err := mr.RemoveModel(ctx, params.Model); err != nil {
			return "", fmt.Errorf("failed to remove model: %w", err)
		}

		return fmt.Sprintf("successfully removed %s", params.Model), nil
	})
}
