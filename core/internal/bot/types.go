package bot

import (
	"context"

	"github.com/bowerhall/sheldon/internal/agent"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot interface {
	Start(ctx context.Context) error
	Send(chatID int64, message string) error
	SendPhoto(chatID int64, data []byte, caption string) error
	SendVideo(chatID int64, data []byte, caption string) error
}

type Config struct {
	Provider    string
	Token       string
	OwnerChatID int64
}

type telegram struct {
	api         *tgbotapi.BotAPI
	agent       *agent.Agent
	ownerChatID int64
}
