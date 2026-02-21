package llm

import "fmt"

func New(cfg Config) (LLM, error) {
	switch cfg.Provider {
	case "claude":
		return newClaude(cfg.APIKey, cfg.Model), nil
	case "openai":
		baseURL := cfg.BaseURL

		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}

		model := cfg.Model
		if model == "" {
			model = "gpt-4o-mini"
		}

		return newOpenAICompatible(cfg.APIKey, baseURL, model), nil
	case "kimi":
		model := cfg.Model

		if model == "" {
			model = "kimi-k2-0711-preview"
		}

		return newOpenAICompatible(cfg.APIKey, "https://api.moonshot.ai/v1", model), nil
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}

		model := cfg.Model
		if model == "" {
			model = "qwen2:0.5b"
		}

		// Ollama's OpenAI-compatible endpoint
		return newOpenAICompatible("ollama", baseURL+"/v1", model), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}
