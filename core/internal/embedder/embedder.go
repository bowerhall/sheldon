package embedder

import (
	"fmt"

	"github.com/bowerhall/sheldonmem"
)

func New(cfg Config) (sheldonmem.Embedder, error) {
	switch cfg.Provider {
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}

		model := cfg.Model
		if model == "" {
			model = "nomic-embed-text"
		}

		return newOllama(baseURL, model), nil
	case "":
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown embedder provider: %s", cfg.Provider)
	}
}
