package bot

import (
	"fmt"

	"github.com/kadet/kora/internal/agent"
)

func New(cfg Config, agent *agent.Agent) (Bot, error) {
	switch cfg.Provider {
	case "telegram":
		return newTelegram(cfg.Token, agent)
	case "discord":
		return nil, fmt.Errorf("discord provider not implemented")
	default:
		return nil, fmt.Errorf("unknown bot provider: %s", cfg.Provider)
	}
}
