package bot

import (
	"context"

	"github.com/kadet/kora/internal/agent"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot interface {
	Start(ctx context.Context) error
}

type Config struct {
	Provider string
	Token    string
}

type telegram struct {
	api   *tgbotapi.BotAPI
	agent *agent.Agent
}
