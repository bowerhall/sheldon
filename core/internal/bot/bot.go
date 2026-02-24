package bot

import (
	"fmt"

	"github.com/bowerhall/sheldon/internal/agent"
)

func New(cfg Config, agent *agent.Agent) (Bot, error) {
	switch cfg.Provider {
	case "telegram":
		return NewTelegram(cfg.Token, agent, cfg.OwnerChatID)
	case "discord":
		return NewDiscord(cfg.Token, agent)
	default:
		return nil, fmt.Errorf("unknown bot provider: %s", cfg.Provider)
	}
}

func NewTelegram(token string, agent *agent.Agent, ownerChatID int64) (Bot, error) {
	return newTelegram(token, agent, ownerChatID)
}

func NewDiscord(token string, agent *agent.Agent) (Bot, error) {
	return newDiscord(token, agent)
}
