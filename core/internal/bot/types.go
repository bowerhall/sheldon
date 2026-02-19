package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kadet/kora/internal/agent"
)

type Bot interface {
	Start(ctx context.Context) error
	Send(chatID int64, message string) error
}

type Config struct {
	Provider string
	Token    string
}

type telegram struct {
	api   *tgbotapi.BotAPI
	agent *agent.Agent
}
