package bot

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/bowerhall/sheldon/internal/agent"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// markdownToTelegramHTML converts common markdown to Telegram-safe HTML
func markdownToTelegramHTML(text string) string {
	// Escape HTML special chars first
	text = html.EscapeString(text)

	// Code blocks: ```code``` → <pre>code</pre>
	codeBlock := regexp.MustCompile("```([\\s\\S]*?)```")
	text = codeBlock.ReplaceAllString(text, "<pre>$1</pre>")

	// Inline code: `code` → <code>code</code>
	inlineCode := regexp.MustCompile("`([^`]+)`")
	text = inlineCode.ReplaceAllString(text, "<code>$1</code>")

	// Bold: **text** or __text__ → <b>text</b>
	bold := regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
	text = bold.ReplaceAllStringFunc(text, func(m string) string {
		inner := regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`).FindStringSubmatch(m)
		if inner[1] != "" {
			return "<b>" + inner[1] + "</b>"
		}
		return "<b>" + inner[2] + "</b>"
	})

	// Italic: *text* or _text_ → <i>text</i>
	italic := regexp.MustCompile(`\*([^*]+)\*|_([^_]+)_`)
	text = italic.ReplaceAllStringFunc(text, func(m string) string {
		inner := regexp.MustCompile(`\*([^*]+)\*|_([^_]+)_`).FindStringSubmatch(m)
		if inner[1] != "" {
			return "<i>" + inner[1] + "</i>"
		}
		return "<i>" + inner[2] + "</i>"
	})

	// Strikethrough: ~~text~~ → <s>text</s>
	strike := regexp.MustCompile(`~~(.+?)~~`)
	text = strike.ReplaceAllString(text, "<s>$1</s>")

	return text
}

func newTelegram(token string, agent *agent.Agent, ownerChatID int64) (Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &telegram{
		api:            api,
		agent:          agent,
		ownerChatID:    ownerChatID,
		activeSessions: make(map[int64]context.CancelFunc),
	}, nil
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
			if update.CallbackQuery != nil {
				go t.handleCallback(update.CallbackQuery)
				continue
			}

			if update.Message == nil {
				continue
			}

			go t.handleMessage(ctx, update.Message)
		}
	}
}

func (t *telegram) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	// Ignore messages from non-owner chats if owner is configured
	if t.ownerChatID != 0 && msg.Chat.ID != t.ownerChatID {
		logger.Warn("ignoring message from unauthorized chat", "chatID", msg.Chat.ID, "from", msg.From.UserName)
		return
	}

	chatID := msg.Chat.ID
	sessionID := fmt.Sprintf("telegram:%d", chatID)

	// Check for stop command
	if isStopCommand(msg.Text) {
		sessionMu.Lock()
		if cancel, ok := t.activeSessions[chatID]; ok {
			cancel()
			delete(t.activeSessions, chatID)
			sessionMu.Unlock()
			logger.Info("operation cancelled by user", "session", sessionID)
			reply := tgbotapi.NewMessage(chatID, "Stopped.")
			t.api.Send(reply)
			return
		}
		sessionMu.Unlock()
		reply := tgbotapi.NewMessage(chatID, "Nothing to stop.")
		t.api.Send(reply)
		return
	}

	// Cancel any existing operation for this chat before starting new one
	sessionMu.Lock()
	if cancel, ok := t.activeSessions[chatID]; ok {
		cancel()
		delete(t.activeSessions, chatID)
	}

	// Create cancellable context for this operation
	opCtx, cancel := context.WithCancel(ctx)
	t.activeSessions[chatID] = cancel
	sessionMu.Unlock()

	// Clean up when done
	defer func() {
		sessionMu.Lock()
		delete(t.activeSessions, chatID)
		sessionMu.Unlock()
	}()

	var media []llm.MediaContent
	var text string

	if msg.Photo != nil && len(msg.Photo) > 0 {
		photo := msg.Photo[len(msg.Photo)-1]

		data, mimeType, err := t.downloadFile(photo.FileID)
		if err != nil {
			logger.Error("failed to download photo", "error", err)
		} else {
			media = append(media, llm.MediaContent{
				Type:     llm.MediaTypeImage,
				Data:     data,
				MimeType: mimeType,
			})
		}

		text = msg.Caption
		logger.Info("photo received", "session", sessionID, "from", msg.From.UserName, "caption", truncate(text, 50))
	} else if msg.Video != nil {
		data, mimeType, err := t.downloadFile(msg.Video.FileID)
		if err != nil {
			logger.Error("failed to download video", "error", err)
		} else {
			media = append(media, llm.MediaContent{
				Type:     llm.MediaTypeVideo,
				Data:     data,
				MimeType: mimeType,
			})
		}

		text = msg.Caption
		logger.Info("video received", "session", sessionID, "from", msg.From.UserName, "caption", truncate(text, 50))
	} else if msg.VideoNote != nil {
		data, mimeType, err := t.downloadFile(msg.VideoNote.FileID)
		if err != nil {
			logger.Error("failed to download video note", "error", err)
		} else {
			media = append(media, llm.MediaContent{
				Type:     llm.MediaTypeVideo,
				Data:     data,
				MimeType: mimeType,
			})
		}

		text = msg.Caption
		logger.Info("video note received", "session", sessionID, "from", msg.From.UserName)
	} else if msg.Document != nil && isPDF(msg.Document.MimeType) {
		data, _, err := t.downloadFile(msg.Document.FileID)
		if err != nil {
			logger.Error("failed to download document", "error", err)
		} else {
			media = append(media, llm.MediaContent{
				Type:     llm.MediaTypePDF,
				Data:     data,
				MimeType: "application/pdf",
			})
		}

		text = msg.Caption
		logger.Info("PDF received", "session", sessionID, "from", msg.From.UserName, "filename", msg.Document.FileName, "caption", truncate(text, 50))
	} else {
		text = msg.Text
		logger.Info("message received", "session", sessionID, "from", msg.From.UserName, "text", truncate(text, 50))
	}

	// send initial "thinking" message for edit-in-place UX
	initialMsg := tgbotapi.NewMessage(chatID, "Thinking...")
	initialMsg.ReplyToMessageID = msg.MessageID
	sent, sendErr := t.api.Send(initialMsg)
	if sendErr != nil {
		logger.Error("failed to send initial message", "error", sendErr)
		// fall back to typing indicator
		t.SendTyping(chatID)
	}
	var progressMsgID int
	if sendErr == nil {
		progressMsgID = sent.MessageID
	}

	// track last status to avoid redundant edits
	var lastStatus string
	var statusMu sync.Mutex

	// progress callback updates the message in place
	onProgress := func(status string) {
		if progressMsgID == 0 {
			return
		}
		statusMu.Lock()
		if status == lastStatus {
			statusMu.Unlock()
			return
		}
		lastStatus = status
		statusMu.Unlock()

		edit := tgbotapi.NewEditMessageText(chatID, progressMsgID, status)
		if _, err := t.api.Send(edit); err != nil {
			logger.Debug("progress edit failed", "error", err)
		}
	}

	response, err := t.agent.ProcessWithOptions(opCtx, sessionID, text, agent.ProcessOptions{
		Media:      media,
		Trusted:    true,
		UserID:     msg.From.ID,
		OnProgress: onProgress,
	})
	if err != nil {
		if opCtx.Err() == context.Canceled {
			logger.Info("operation was cancelled", "session", sessionID)
			// edit the progress message to show cancelled
			if progressMsgID != 0 {
				edit := tgbotapi.NewEditMessageText(chatID, progressMsgID, "Cancelled.")
				t.api.Send(edit)
			}
			return
		}
		logger.Error("agent failed", "error", err)
		response = "Something went wrong."
	}

	// final edit with the complete response
	if progressMsgID != 0 {
		edit := tgbotapi.NewEditMessageText(chatID, progressMsgID, markdownToTelegramHTML(response))
		edit.ParseMode = tgbotapi.ModeHTML
		if _, err := t.api.Send(edit); err != nil {
			logger.Error("final edit failed", "error", err)
			// fall back to sending a new message
			reply := tgbotapi.NewMessage(chatID, markdownToTelegramHTML(response))
			reply.ParseMode = tgbotapi.ModeHTML
			t.api.Send(reply)
		} else {
			logger.Info("reply sent (edited)", "chars", len(response))
		}
	} else {
		reply := tgbotapi.NewMessage(chatID, markdownToTelegramHTML(response))
		reply.ReplyToMessageID = msg.MessageID
		reply.ParseMode = tgbotapi.ModeHTML
		if _, err := t.api.Send(reply); err != nil {
			logger.Error("send failed", "error", err)
		} else {
			logger.Info("reply sent", "chars", len(response))
		}
	}
}

func (t *telegram) Send(chatID int64, message string) error {
	msg := tgbotapi.NewMessage(chatID, markdownToTelegramHTML(message))
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := t.api.Send(msg)
	if err != nil {
		logger.Error("proactive send failed", "error", err, "chatID", chatID)
	} else {
		logger.Info("proactive message sent", "chatID", chatID, "chars", len(message))
	}
	return err
}

func (t *telegram) SendTyping(chatID int64) error {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	_, err := t.api.Request(action)
	return err
}

func (t *telegram) downloadFile(fileID string) ([]byte, string, error) {
	file, err := t.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, "", err
	}

	url := file.Link(t.api.Token)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMediaSize))
	if err != nil {
		return nil, "", err
	}

	mimeType := http.DetectContentType(data)

	return data, mimeType, nil
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

func (t *telegram) SendVideo(chatID int64, data []byte, caption string) error {
	videoBytes := tgbotapi.FileBytes{Name: "video.mp4", Bytes: data}
	msg := tgbotapi.NewVideo(chatID, videoBytes)
	msg.Caption = caption
	_, err := t.api.Send(msg)
	if err != nil {
		logger.Error("send video failed", "error", err, "chatID", chatID)
	} else {
		logger.Info("video sent", "chatID", chatID, "caption", truncate(caption, 50))
	}
	return err
}

func (t *telegram) SendDocument(chatID int64, data []byte, filename, caption string) error {
	docBytes := tgbotapi.FileBytes{Name: filename, Bytes: data}
	msg := tgbotapi.NewDocument(chatID, docBytes)
	msg.Caption = caption
	_, err := t.api.Send(msg)
	if err != nil {
		logger.Error("send document failed", "error", err, "chatID", chatID)
	} else {
		logger.Info("document sent", "chatID", chatID, "filename", filename, "caption", truncate(caption, 50))
	}
	return err
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}

	return s[:max] + "..."
}

func isPDF(mimeType string) bool {
	return mimeType == "application/pdf"
}

func (t *telegram) SendWithButtons(chatID int64, message string, buttons []Button) (int64, error) {
	var keyboardButtons []tgbotapi.InlineKeyboardButton
	for _, b := range buttons {
		keyboardButtons = append(keyboardButtons, tgbotapi.NewInlineKeyboardButtonData(b.Label, b.CallbackID))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(keyboardButtons...),
	)

	msg := tgbotapi.NewMessage(chatID, markdownToTelegramHTML(message))
	msg.ReplyMarkup = keyboard
	msg.ParseMode = tgbotapi.ModeHTML

	sent, err := t.api.Send(msg)
	if err != nil {
		logger.Error("send with buttons failed", "error", err, "chatID", chatID)
		return 0, err
	}

	logger.Info("message with buttons sent", "chatID", chatID, "messageID", sent.MessageID)
	return int64(sent.MessageID), nil
}

func (t *telegram) SetApprovalCallback(fn ApprovalCallback) {
	t.approvalCallback = fn
}

func (t *telegram) handleCallback(callback *tgbotapi.CallbackQuery) {
	if t.approvalCallback == nil {
		logger.Warn("received callback but no handler set", "data", callback.Data)
		return
	}

	data := callback.Data
	userID := callback.From.ID

	var approvalID string
	var approved bool

	if len(data) > 8 && data[len(data)-8:] == ":approve" {
		approvalID = data[:len(data)-8]
		approved = true
	} else if len(data) > 5 && data[len(data)-5:] == ":deny" {
		approvalID = data[:len(data)-5]
		approved = false
	} else {
		logger.Warn("unknown callback format", "data", data)
		return
	}

	t.approvalCallback(approvalID, approved, userID)

	answer := tgbotapi.NewCallback(callback.ID, "")
	t.api.Request(answer)

	var resultText string
	if approved {
		resultText = "Approved"
	} else {
		resultText = "Denied"
	}
	edit := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, callback.Message.Text+"\n\n"+resultText)
	t.api.Send(edit)
}
