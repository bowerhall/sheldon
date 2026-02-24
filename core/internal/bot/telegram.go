package bot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bowerhall/sheldon/internal/agent"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const maxImageSize = 20 * 1024 * 1024 // 20MB limit for images

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

	var images []llm.ImageContent
	var text string

	if msg.Photo != nil && len(msg.Photo) > 0 {
		photo := msg.Photo[len(msg.Photo)-1]

		imgData, mediaType, err := t.downloadFile(photo.FileID)
		if err != nil {
			logger.Error("failed to download photo", "error", err)
		} else {
			images = append(images, llm.ImageContent{
				Data:      imgData,
				MediaType: mediaType,
			})
		}

		text = msg.Caption
		logger.Info("photo received", "session", sessionID, "from", msg.From.UserName, "caption", truncate(text, 50))
	} else {
		text = msg.Text
		logger.Info("message received", "session", sessionID, "from", msg.From.UserName, "text", truncate(text, 50))
	}

	response, err := t.agent.ProcessWithImages(ctx, sessionID, text, images)
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

func (t *telegram) downloadFile(fileID string) ([]byte, string, error) {
	file, err := t.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, "", err
	}

	url := file.Link(t.api.Token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize))
	if err != nil {
		return nil, "", err
	}

	mediaType := http.DetectContentType(data)

	return data, mediaType, nil
}

func (t *telegram) SendPhoto(chatID int64, data []byte, caption string) error {
	photoBytes := tgbotapi.FileBytes{Name: "image", Bytes: data}
	msg := tgbotapi.NewPhoto(chatID, photoBytes)
	msg.Caption = caption
	_, err := t.api.Send(msg)
	if err != nil {
		logger.Error("send photo failed", "error", err, "chatID", chatID)
	} else {
		logger.Info("photo sent", "chatID", chatID, "caption", truncate(caption, 50))
	}
	return err
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}

	return s[:max] + "..."
}
