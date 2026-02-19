package bot

import (
	"fmt"

	"github.com/kadet/kora/internal/agent"
)

func New(cfg Config, agent *agent.Agent) (Bot, error) {
	switch cfg.Provider {
	case "telegram":
		return NewTelegram(cfg.Token, agent)
	case "discord":
		return NewDiscord(cfg.Token, agent)
	default:
		return nil, fmt.Errorf("unknown bot provider: %s", cfg.Provider)
	}
}

func NewTelegram(token string, agent *agent.Agent) (Bot, error) {
	return newTelegram(token, agent)
}

func NewDiscord(token string, agent *agent.Agent) (Bot, error) {
	return newDiscord(token, agent)
}
