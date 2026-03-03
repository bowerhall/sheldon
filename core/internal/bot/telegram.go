package bot

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/bowerhall/sheldon/internal/agent"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/logger"
	"github.com/bowerhall/sheldon/internal/transcribe"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// markdownToTelegramHTML converts common markdown to Telegram-safe HTML
// Uses placeholder approach to prevent formatting inside code blocks and URLs
func markdownToTelegramHTML(text string) string {
	// Escape HTML special chars first
	text = html.EscapeString(text)

	// Extract code blocks, inline code, and URLs - replace with placeholders
	var codeBlocks []string
	var inlineCodes []string
	var urls []string

	// Code blocks: ```code``` → placeholder
	codeBlock := regexp.MustCompile("```([\\s\\S]*?)```")
	text = codeBlock.ReplaceAllStringFunc(text, func(m string) string {
		inner := codeBlock.FindStringSubmatch(m)[1]
		codeBlocks = append(codeBlocks, inner)
		return fmt.Sprintf("\x00CODEBLOCK%d\x00", len(codeBlocks)-1)
	})

	// Inline code: `code` → placeholder
	inlineCode := regexp.MustCompile("`([^`]+)`")
	text = inlineCode.ReplaceAllStringFunc(text, func(m string) string {
		inner := inlineCode.FindStringSubmatch(m)[1]
		inlineCodes = append(inlineCodes, inner)
		return fmt.Sprintf("\x00INLINE%d\x00", len(inlineCodes)-1)
	})

	// URLs: protect from underscore/asterisk formatting
	// Exclude * to avoid capturing markdown bold markers like **url**
	urlPattern := regexp.MustCompile(`https?://[^\s<>"*]+`)
	text = urlPattern.ReplaceAllStringFunc(text, func(m string) string {
		urls = append(urls, m)
		return fmt.Sprintf("\x00URL%d\x00", len(urls)-1)
	})

	// Now process formatting on text without code
	// Bold: **text** or __text__ → <b>text</b>
	bold := regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
	text = bold.ReplaceAllStringFunc(text, func(m string) string {
		inner := bold.FindStringSubmatch(m)
		if inner[1] != "" {
			return "<b>" + inner[1] + "</b>"
		}
		return "<b>" + inner[2] + "</b>"
	})

	// Italic: *text* or _text_ → <i>text</i>
	italic := regexp.MustCompile(`\*([^*]+)\*|_([^_]+)_`)
	text = italic.ReplaceAllStringFunc(text, func(m string) string {
		inner := italic.FindStringSubmatch(m)
		if inner[1] != "" {
			return "<i>" + inner[1] + "</i>"
		}
		return "<i>" + inner[2] + "</i>"
	})

	// Strikethrough: ~~text~~ → <s>text</s>
	strike := regexp.MustCompile(`~~(.+?)~~`)
	text = strike.ReplaceAllString(text, "<s>$1</s>")

	// Restore code blocks and URLs
	for i, code := range codeBlocks {
		text = regexp.MustCompile(fmt.Sprintf("\x00CODEBLOCK%d\x00", i)).ReplaceAllString(text, "<pre>"+code+"</pre>")
	}
	for i, code := range inlineCodes {
		text = regexp.MustCompile(fmt.Sprintf("\x00INLINE%d\x00", i)).ReplaceAllString(text, "<code>"+code+"</code>")
	}
	for i, url := range urls {
		text = regexp.MustCompile(fmt.Sprintf("\x00URL%d\x00", i)).ReplaceAllString(text, url)
	}

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
	} else if msg.Voice != nil {
		data, mimeType, err := t.downloadFile(msg.Voice.FileID)
		if err != nil {
			logger.Error("failed to download voice", "error", err)
		} else {
			transcription, err := transcribe.Transcribe(data, mimeType)
			if err != nil {
				logger.Error("failed to transcribe voice", "error", err)
				text = "[Voice message - transcription failed]"
			} else {
				text = transcription
				logger.Info("voice transcribed", "session", sessionID, "from", msg.From.UserName, "duration", msg.Voice.Duration, "chars", len(transcription))
			}
		}
	} else {
		text = msg.Text
		logger.Info("message received", "session", sessionID, "from", msg.From.UserName, "text", truncate(text, 50))
	}

	// send typing indicator while processing
	t.SendTyping(chatID)
	typingDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-typingDone:
				return
			case <-opCtx.Done():
				return
			case <-ticker.C:
				t.SendTyping(chatID)
			}
		}
	}()

	response, err := t.agent.ProcessWithOptions(opCtx, sessionID, text, agent.ProcessOptions{
		Media:   media,
		Trusted: true,
		UserID:  msg.From.ID,
	})
	close(typingDone)
	if err != nil {
		if opCtx.Err() == context.Canceled {
			logger.Info("operation was cancelled", "session", sessionID)
			return // Don't send error message, user already got "Stopped."
		}
		logger.Error("agent failed", "error", err)
		response = "Something went wrong."
	}

	reply := tgbotapi.NewMessage(chatID, markdownToTelegramHTML(response))
	reply.ReplyToMessageID = msg.MessageID
	reply.ParseMode = tgbotapi.ModeHTML

	if _, err := t.api.Send(reply); err != nil {
		logger.Error("send failed", "error", err)
	} else {
		logger.Info("reply sent", "chars", len(response))
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
