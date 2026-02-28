package bot

import (
	"context"

	"github.com/bowerhall/sheldon/internal/agent"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot interface {
	Start(ctx context.Context) error
	Send(chatID int64, message string) error
	SendTyping(chatID int64) error
	SendPhoto(chatID int64, data []byte, caption string) error
	SendVideo(chatID int64, data []byte, caption string) error
	SendDocument(chatID int64, data []byte, filename, caption string) error
	SendWithButtons(chatID int64, message string, buttons []Button) (messageID int64, err error)
	SetApprovalCallback(fn ApprovalCallback)
}

type Button struct {
	Label      string
	CallbackID string
}

type ApprovalCallback func(approvalID string, approved bool, userID int64)

type Config struct {
	Provider       string
	Token          string
	OwnerChatID    int64  // Telegram: restrict to this chat ID
	GuildID        string // Discord: restrict to this guild/server ID
	OwnerID        string // Discord: user ID with full access (sensitive facts)
	TrustedChannel string // Discord: channel ID with full access
}

type telegram struct {
	api              *tgbotapi.BotAPI
	agent            *agent.Agent
	ownerChatID      int64
	activeSessions   map[int64]context.CancelFunc
	approvalCallback ApprovalCallback
}
