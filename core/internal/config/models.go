package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type ModelInfo struct {
	ID           string   `json:"id"`
	Provider     string   `json:"provider"`
	Name         string   `json:"name"`
	Local        bool     `json:"local"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type ModelRegistry struct {
	runtimeConfig *RuntimeConfig
	client        *http.Client
}

func NewModelRegistry(rc *RuntimeConfig) *ModelRegistry {
	return &ModelRegistry{
		runtimeConfig: rc,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (r *ModelRegistry) ollamaURL() string {
	if r.runtimeConfig != nil {
		return r.runtimeConfig.Get("ollama_host")
	}
	return "http://localhost:11434"
}

func (r *ModelRegistry) CloudModels() []ModelInfo {
	return []ModelInfo{
		{ID: "kimi-k2-0711-preview", Provider: "kimi", Name: "Kimi K2 Preview", Local: false, Capabilities: []string{"chat", "tools"}},
		{ID: "kimi-k2.5:cloud", Provider: "kimi", Name: "Kimi K2.5 Cloud", Local: false, Capabilities: []string{"chat", "tools", "code"}},
		{ID: "claude-sonnet-4-20250514", Provider: "claude", Name: "Claude Sonnet 4", Local: false, Capabilities: []string{"chat", "tools", "code"}},
		{ID: "claude-opus-4-5-20251101", Provider: "claude", Name: "Claude Opus 4.5", Local: false, Capabilities: []string{"chat", "tools", "code"}},
		{ID: "gpt-4o", Provider: "openai", Name: "GPT-4o", Local: false, Capabilities: []string{"chat", "tools"}},
		{ID: "gpt-4o-mini", Provider: "openai", Name: "GPT-4o Mini", Local: false, Capabilities: []string{"chat", "tools"}},
	}
}

type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

type ollamaModel struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

func (r *ModelRegistry) LocalModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", r.ollamaURL()+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ModelInfo, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		models = append(models, ModelInfo{
			ID:       m.Name,
			Provider: "ollama",
			Name:     m.Name,
			Local:    true,
		})
	}

	return models, nil
}

func (r *ModelRegistry) AllModels(ctx context.Context) ([]ModelInfo, error) {
	models := r.CloudModels()

	localModels, err := r.LocalModels(ctx)
	if err != nil {
		return models, nil
	}

	models = append(models, localModels...)
	return models, nil
}

func (r *ModelRegistry) ModelsByProvider(ctx context.Context, provider string) ([]ModelInfo, error) {
	all, err := r.AllModels(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []ModelInfo
	for _, m := range all {
		if m.Provider == provider {
			filtered = append(filtered, m)
		}
	}

	return filtered, nil
}

type ollamaPullRequest struct {
	Name   string `json:"name"`
	Stream bool   `json:"stream"`
}

type ollamaPullResponse struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

func (r *ModelRegistry) PullModel(ctx context.Context, name string) error {
	reqBody, err := json.Marshal(ollamaPullRequest{Name: name, Stream: false})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.ollamaURL()+"/api/pull", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// use a streaming request with longer timeout for pulls
	pullClient := &http.Client{Timeout: 30 * time.Minute}

	req, err = http.NewRequestWithContext(ctx, "POST", r.ollamaURL()+"/api/pull", io.NopCloser(
		&jsonReader{data: reqBody},
	))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := pullClient.Do(req)
	if err != nil {
		return fmt.Errorf("pull request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	// read through streaming response until complete
	decoder := json.NewDecoder(resp.Body)
	for {
		var pullResp ollamaPullResponse
		if err := decoder.Decode(&pullResp); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		if pullResp.Status == "success" {
			return nil
		}
	}

	return nil
}

type jsonReader struct {
	data []byte
	pos  int
}

func (r *jsonReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *ModelRegistry) Providers() []ProviderInfo {
	return []ProviderInfo{
		// Core providers
		{ID: "kimi", Name: "Moonshot Kimi", EnvKey: "KIMI_API_KEY"},
		{ID: "claude", Name: "Anthropic Claude", EnvKey: "ANTHROPIC_API_KEY"},
		{ID: "openai", Name: "OpenAI", EnvKey: "OPENAI_API_KEY"},
		{ID: "nvidia", Name: "NVIDIA NIM", EnvKey: "NVIDIA_API_KEY"},
		{ID: "ollama", Name: "Ollama (local)", EnvKey: ""},
		// OpenAI-compatible providers (add API key to Doppler to enable)
		{ID: "mistral", Name: "Mistral AI", EnvKey: "MISTRAL_API_KEY"},
		{ID: "groq", Name: "Groq", EnvKey: "GROQ_API_KEY"},
		{ID: "together", Name: "Together AI", EnvKey: "TOGETHER_API_KEY"},
		{ID: "deepseek", Name: "DeepSeek", EnvKey: "DEEPSEEK_API_KEY"},
		{ID: "fireworks", Name: "Fireworks AI", EnvKey: "FIREWORKS_API_KEY"},
		{ID: "perplexity", Name: "Perplexity", EnvKey: "PERPLEXITY_API_KEY"},
	}
}

// EnvKeyForProvider returns the environment variable name for a provider's API key
func EnvKeyForProvider(provider string) string {
	switch provider {
	case "claude":
		return "ANTHROPIC_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	case "kimi":
		return "KIMI_API_KEY"
	case "nvidia":
		return "NVIDIA_API_KEY"
	case "ollama":
		return ""
	default:
		return strings.ToUpper(provider) + "_API_KEY"
	}
}

// InferProviderFromModel guesses the provider from a model name
func InferProviderFromModel(model string) string {
	switch {
	case strings.HasPrefix(model, "kimi-") || strings.Contains(model, "kimi"):
		return "kimi"
	case strings.HasPrefix(model, "claude-"):
		return "claude"
	case strings.HasPrefix(model, "gpt-"):
		return "openai"
	case strings.Contains(model, "llama") || strings.Contains(model, "mistral") || strings.Contains(model, "qwen"):
		return "ollama"
	default:
		return ""
	}
}

type ProviderInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	EnvKey string `json:"env_key,omitempty"`
}

func (r *ModelRegistry) ProviderStatus(ctx context.Context) []ProviderStatus {
	providers := r.Providers()
	statuses := make([]ProviderStatus, 0, len(providers))

	for _, p := range providers {
		status := ProviderStatus{
			ProviderInfo: p,
			Available:    false,
		}

		if p.ID == "ollama" {
			_, err := r.LocalModels(ctx)
			status.Available = err == nil
		} else {
			status.Available = true
		}

		statuses = append(statuses, status)
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].ID < statuses[j].ID
	})

	return statuses
}

type ProviderStatus struct {
	ProviderInfo
	Available bool `json:"available"`
}
