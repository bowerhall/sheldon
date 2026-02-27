package embedder

import (
	"fmt"

	"github.com/bowerhall/sheldonmem"
	"github.com/bowerhall/sheldonmem/ollama"
)

func New(cfg Config) (sheldonmem.Embedder, error) {
	switch cfg.Provider {
	case "ollama":
		return ollama.NewEmbedder(cfg.BaseURL, cfg.Model), nil
	case "":
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown embedder provider: %s", cfg.Provider)
	}
}
