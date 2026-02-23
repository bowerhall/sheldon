package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RuntimeConfig holds config values that can be changed at runtime
// Only non-secret values are allowed
type RuntimeConfig struct {
	mu   sync.RWMutex
	path string
	data RuntimeData
}

// RuntimeData is the serializable runtime config
type RuntimeData struct {
	LLMProvider       string `json:"llm_provider,omitempty"`
	LLMModel          string `json:"llm_model,omitempty"`
	ExtractorProvider string `json:"extractor_provider,omitempty"`
	ExtractorModel    string `json:"extractor_model,omitempty"`
	EmbedderProvider  string `json:"embedder_provider,omitempty"`
	EmbedderModel     string `json:"embedder_model,omitempty"`
	CoderProvider     string `json:"coder_provider,omitempty"`
	CoderModel        string `json:"coder_model,omitempty"`
	OllamaHost        string `json:"ollama_host,omitempty"`
}

// AllowedKeys defines which config keys can be changed at runtime
var AllowedKeys = map[string]string{
	"llm_provider":       "LLM provider (e.g., kimi, claude, openai, ollama)",
	"llm_model":          "LLM model name (e.g., kimi-k2-0711-preview, claude-sonnet-4-20250514)",
	"extractor_provider": "Extractor provider for memory extraction",
	"extractor_model":    "Extractor model name",
	"embedder_provider":  "Embedder provider (e.g., ollama, voyage)",
	"embedder_model":     "Embedder model name (e.g., nomic-embed-text)",
	"coder_provider":     "Coder provider for code generation (e.g., kimi, claude)",
	"coder_model":        "Coder model name (e.g., kimi-k2-0711-preview)",
	"ollama_host":        "Ollama server URL (e.g., http://localhost:11434, http://gpu-monster:11434)",
}

// NewRuntimeConfig creates a runtime config, loading from file if exists
func NewRuntimeConfig(dataDir string) (*RuntimeConfig, error) {
	rc := &RuntimeConfig{
		path: filepath.Join(dataDir, "runtime_config.json"),
	}

	// load existing config if present
	if data, err := os.ReadFile(rc.path); err == nil {
		json.Unmarshal(data, &rc.data)
	}

	// validate and auto-fix invalid configs
	rc.validateAndFix()

	return rc, nil
}

// validateAndFix checks for invalid model configurations and resets them
func (rc *RuntimeConfig) validateAndFix() {
	changed := false

	// check if llm_model is a coder-only model (doesn't have chat capability)
	if rc.data.LLMModel != "" {
		if !modelHasCapability(rc.data.LLMModel, "chat") {
			rc.data.LLMModel = ""
			rc.data.LLMProvider = ""
			changed = true
		}
	}

	if changed {
		rc.save()
	}
}

// modelHasCapability checks if a known model has a specific capability
func modelHasCapability(modelID, capability string) bool {
	// known models and their capabilities
	models := map[string][]string{
		"kimi-k2-0711-preview":      {"chat", "tools"},
		"kimi-k2.5:cloud":           {"code"}, // coder only, no chat
		"claude-sonnet-4-20250514":  {"chat", "tools", "code"},
		"claude-opus-4-5-20251101":  {"chat", "tools", "code"},
		"gpt-4o":                    {"chat", "tools"},
		"gpt-4o-mini":               {"chat", "tools"},
	}

	caps, known := models[modelID]
	if !known {
		// unknown model - allow it (could be ollama or new model)
		return true
	}

	for _, c := range caps {
		if c == capability {
			return true
		}
	}
	return false
}

// Get returns a runtime config value, falling back to env var if not set
func (rc *RuntimeConfig) Get(key string) string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	switch key {
	case "llm_provider":
		if rc.data.LLMProvider != "" {
			return rc.data.LLMProvider
		}
		return os.Getenv("LLM_PROVIDER")
	case "llm_model":
		if rc.data.LLMModel != "" {
			return rc.data.LLMModel
		}
		return os.Getenv("LLM_MODEL")
	case "extractor_provider":
		if rc.data.ExtractorProvider != "" {
			return rc.data.ExtractorProvider
		}
		return os.Getenv("EXTRACTOR_PROVIDER")
	case "extractor_model":
		if rc.data.ExtractorModel != "" {
			return rc.data.ExtractorModel
		}
		return os.Getenv("EXTRACTOR_MODEL")
	case "embedder_provider":
		if rc.data.EmbedderProvider != "" {
			return rc.data.EmbedderProvider
		}
		return os.Getenv("EMBEDDER_PROVIDER")
	case "embedder_model":
		if rc.data.EmbedderModel != "" {
			return rc.data.EmbedderModel
		}
		return os.Getenv("EMBEDDER_MODEL")
	case "coder_provider":
		if rc.data.CoderProvider != "" {
			return rc.data.CoderProvider
		}
		return os.Getenv("CODER_PROVIDER")
	case "coder_model":
		if rc.data.CoderModel != "" {
			return rc.data.CoderModel
		}
		return os.Getenv("CODER_MODEL")
	case "ollama_host":
		if rc.data.OllamaHost != "" {
			return rc.data.OllamaHost
		}
		host := os.Getenv("OLLAMA_HOST")
		if host == "" {
			return "http://localhost:11434"
		}
		return host
	}
	return ""
}

// Set updates a runtime config value
func (rc *RuntimeConfig) Set(key, value string) error {
	if _, ok := AllowedKeys[key]; !ok {
		return fmt.Errorf("key %q is not allowed for runtime config", key)
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	switch key {
	case "llm_provider":
		rc.data.LLMProvider = value
	case "llm_model":
		rc.data.LLMModel = value
	case "extractor_provider":
		rc.data.ExtractorProvider = value
	case "extractor_model":
		rc.data.ExtractorModel = value
	case "embedder_provider":
		rc.data.EmbedderProvider = value
	case "embedder_model":
		rc.data.EmbedderModel = value
	case "coder_provider":
		rc.data.CoderProvider = value
	case "coder_model":
		rc.data.CoderModel = value
	case "ollama_host":
		rc.data.OllamaHost = value
	default:
		return fmt.Errorf("unknown key: %s", key)
	}

	return rc.save()
}

// Reset clears a runtime config value, reverting to env var
func (rc *RuntimeConfig) Reset(key string) error {
	return rc.Set(key, "")
}

// ResetAll clears all runtime config values
func (rc *RuntimeConfig) ResetAll() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.data = RuntimeData{}
	return rc.save()
}

// All returns all current runtime values (with env fallbacks)
func (rc *RuntimeConfig) All() map[string]string {
	result := make(map[string]string)
	for key := range AllowedKeys {
		result[key] = rc.Get(key)
	}
	return result
}

// Overrides returns only the values that override env vars
func (rc *RuntimeConfig) Overrides() map[string]string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	result := make(map[string]string)
	if rc.data.LLMProvider != "" {
		result["llm_provider"] = rc.data.LLMProvider
	}
	if rc.data.LLMModel != "" {
		result["llm_model"] = rc.data.LLMModel
	}
	if rc.data.ExtractorProvider != "" {
		result["extractor_provider"] = rc.data.ExtractorProvider
	}
	if rc.data.ExtractorModel != "" {
		result["extractor_model"] = rc.data.ExtractorModel
	}
	if rc.data.EmbedderProvider != "" {
		result["embedder_provider"] = rc.data.EmbedderProvider
	}
	if rc.data.EmbedderModel != "" {
		result["embedder_model"] = rc.data.EmbedderModel
	}
	if rc.data.CoderProvider != "" {
		result["coder_provider"] = rc.data.CoderProvider
	}
	if rc.data.CoderModel != "" {
		result["coder_model"] = rc.data.CoderModel
	}
	if rc.data.OllamaHost != "" {
		result["ollama_host"] = rc.data.OllamaHost
	}
	return result
}

func (rc *RuntimeConfig) save() error {
	data, err := json.MarshalIndent(rc.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rc.path, data, 0644)
}
