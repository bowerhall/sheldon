package bot

import (
	"context"
	"fmt"

	"github.com/bowerhall/sheldon/internal/agent"
	"github.com/bowerhall/sheldon/internal/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func newTelegram(token string, agent *agent.Agent) (Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &telegram{api: api, agent: agent}, nil
}

func (t *telegram) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := t.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			go t.handleMessage(ctx, update.Message)
		}
	}
}

func (t *telegram) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	sessionID := fmt.Sprintf("telegram:%d", msg.Chat.ID)
	logger.Info("message received", "session", sessionID, "from", msg.From.UserName, "text", truncate(msg.Text, 50))

	response, err := t.agent.Process(ctx, sessionID, msg.Text)
	if err != nil {
		logger.Error("agent failed", "error", err)
		response = "Something went wrong."
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, response)
	reply.ReplyToMessageID = msg.MessageID

	if _, err := t.api.Send(reply); err != nil {
		logger.Error("send failed", "error", err)
	} else {
		logger.Info("reply sent", "chars", len(response))
	}
}

func (t *telegram) Send(chatID int64, message string) error {
	msg := tgbotapi.NewMessage(chatID, message)
	_, err := t.api.Send(msg)
	if err != nil {
		logger.Error("proactive send failed", "error", err, "chatID", chatID)
	} else {
		logger.Info("proactive message sent", "chatID", chatID, "chars", len(message))
	}
	return err
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}

	return s[:max] + "..."
}
