package llm

import "fmt"

// OpenAI-compatible providers and their base URLs
// To add a new provider: add entry here + add {PROVIDER}_API_KEY to Doppler
var openAICompatibleProviders = map[string]string{
	"mistral":    "https://api.mistral.ai/v1",
	"groq":       "https://api.groq.com/openai/v1",
	"together":   "https://api.together.xyz/v1",
	"deepseek":   "https://api.deepseek.com/v1",
	"fireworks":  "https://api.fireworks.ai/inference/v1",
	"perplexity": "https://api.perplexity.ai",
}

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
		// check if it's an OpenAI-compatible provider
		if baseURL, ok := openAICompatibleProviders[cfg.Provider]; ok {
			if cfg.BaseURL != "" {
				baseURL = cfg.BaseURL
			}
			return newOpenAICompatible(cfg.APIKey, baseURL, cfg.Model), nil
		}
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

// KnownProviders returns all known provider IDs
func KnownProviders() []string {
	providers := []string{"claude", "openai", "kimi", "ollama"}
	for p := range openAICompatibleProviders {
		providers = append(providers, p)
	}
	return providers
}

// IsKnownProvider checks if a provider is recognized
func IsKnownProvider(provider string) bool {
	switch provider {
	case "claude", "openai", "kimi", "ollama":
		return true
	default:
		_, ok := openAICompatibleProviders[provider]
		return ok
	}
}
