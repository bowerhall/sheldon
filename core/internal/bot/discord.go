package bot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/agent"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/logger"
	"github.com/bwmarrin/discordgo"
)

type discord struct {
	session        *discordgo.Session
	agent          *agent.Agent
	guildID        string
	ownerID        string
	trustedChannel string
	ctx            context.Context
	activeSessions map[string]context.CancelFunc
}

func newDiscord(token string, agent *agent.Agent, guildID, ownerID, trustedChannel string) (Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	d := &discord{
		session:        session,
		agent:          agent,
		guildID:        guildID,
		ownerID:        ownerID,
		trustedChannel: trustedChannel,
		activeSessions: make(map[string]context.CancelFunc),
	}

	session.AddHandler(d.handleMessage)

	return d, nil
}

func (d *discord) Start(ctx context.Context) error {
	d.ctx = ctx

	if err := d.session.Open(); err != nil {
		return err
	}

	<-ctx.Done()
	return d.session.Close()
}

func (d *discord) Send(chatID int64, message string) error {
	channelID := fmt.Sprintf("%d", chatID)
	_, err := d.session.ChannelMessageSend(channelID, message)
	if err != nil {
		logger.Error("discord send failed", "error", err, "channelID", channelID)
	} else {
		logger.Info("discord message sent", "channelID", channelID, "chars", len(message))
	}
	return err
}

func (d *discord) SendPhoto(chatID int64, data []byte, caption string) error {
	channelID := fmt.Sprintf("%d", chatID)
	reader := bytes.NewReader(data)
	_, err := d.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: caption,
		Files: []*discordgo.File{
			{
				Name:   "image.png",
				Reader: reader,
			},
		},
	})
	if err != nil {
		logger.Error("discord send photo failed", "error", err, "channelID", channelID)
	} else {
		logger.Info("discord photo sent", "channelID", channelID)
	}
	return err
}

func (d *discord) SendVideo(chatID int64, data []byte, caption string) error {
	channelID := fmt.Sprintf("%d", chatID)
	reader := bytes.NewReader(data)
	_, err := d.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: caption,
		Files: []*discordgo.File{
			{
				Name:   "video.mp4",
				Reader: reader,
			},
		},
	})
	if err != nil {
		logger.Error("discord send video failed", "error", err, "channelID", channelID)
	} else {
		logger.Info("discord video sent", "channelID", channelID)
	}
	return err
}

func (d *discord) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Allow owner DMs even if guild restriction is set
	isOwnerDM := m.GuildID == "" && d.ownerID != "" && m.Author.ID == d.ownerID

	// Guild restriction (skip for owner DMs)
	if !isOwnerDM && d.guildID != "" && m.GuildID != d.guildID {
		logger.Warn("ignoring message from unauthorized guild", "guildID", m.GuildID, "from", m.Author.Username)
		return
	}

	go d.processMessage(s, m)
}

func (d *discord) processMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	channelID := m.ChannelID
	sessionID := fmt.Sprintf("discord:%s", channelID)

	// Check for stop command
	if isStopCommand(m.Content) {
		sessionMu.Lock()
		if cancel, ok := d.activeSessions[channelID]; ok {
			cancel()
			delete(d.activeSessions, channelID)
			sessionMu.Unlock()
			logger.Info("operation cancelled by user", "session", sessionID)
			s.ChannelMessageSend(channelID, "Stopped.")
			return
		}
		sessionMu.Unlock()
		s.ChannelMessageSend(channelID, "Nothing to stop.")
		return
	}

	// Cancel any existing operation for this channel before starting new one
	sessionMu.Lock()
	if cancel, ok := d.activeSessions[channelID]; ok {
		cancel()
		delete(d.activeSessions, channelID)
	}

	// Create cancellable context for this operation
	opCtx, cancel := context.WithCancel(d.ctx)
	d.activeSessions[channelID] = cancel
	sessionMu.Unlock()

	// Clean up when done
	defer func() {
		sessionMu.Lock()
		delete(d.activeSessions, channelID)
		sessionMu.Unlock()
	}()

	var media []llm.MediaContent
	text := m.Content

	// Download attachments (images/videos)
	for _, att := range m.Attachments {
		if att.Size > maxMediaSize {
			logger.Warn("attachment too large, skipping", "size", att.Size, "max", maxMediaSize)
			continue
		}

		data, mimeType, err := d.downloadAttachment(att.URL)
		if err != nil {
			logger.Error("failed to download attachment", "error", err, "url", att.URL)
			continue
		}

		mediaType := llm.MediaTypeImage
		if strings.HasPrefix(mimeType, "video/") {
			mediaType = llm.MediaTypeVideo
		}

		if mediaType == llm.MediaTypeImage || mediaType == llm.MediaTypeVideo {
			media = append(media, llm.MediaContent{
				Type:     mediaType,
				Data:     data,
				MimeType: mimeType,
			})
			logger.Info("attachment received", "type", mediaType, "size", len(data))
		}
	}

	// Determine if this is a trusted context (can access sensitive facts)
	trusted := d.isTrusted(m)

	if len(media) > 0 {
		logger.Info("message with media received", "session", sessionID, "from", m.Author.Username, "attachments", len(media), "trusted", trusted)
	} else {
		logger.Info("message received", "session", sessionID, "from", m.Author.Username, "text", truncate(text, 50), "trusted", trusted)
	}

	response, err := d.agent.ProcessWithOptions(opCtx, sessionID, text, agent.ProcessOptions{
		Media:   media,
		Trusted: trusted,
	})
	if err != nil {
		if opCtx.Err() == context.Canceled {
			logger.Info("operation was cancelled", "session", sessionID)
			return
		}
		logger.Error("agent failed", "error", err)
		response = "Something went wrong."
	}

	if _, err := s.ChannelMessageSendReply(m.ChannelID, response, m.Reference()); err != nil {
		logger.Error("discord reply failed", "error", err)
	} else {
		logger.Info("reply sent", "chars", len(response))
	}
}

// isTrusted returns true if the message is from a trusted source (owner DM or trusted channel)
func (d *discord) isTrusted(m *discordgo.MessageCreate) bool {
	// Owner DM: no guild ID means DM, and author matches owner
	if d.ownerID != "" && m.GuildID == "" && m.Author.ID == d.ownerID {
		return true
	}

	// Trusted channel
	if d.trustedChannel != "" && m.ChannelID == d.trustedChannel {
		return true
	}

	// No restrictions configured = trusted (backwards compatible)
	if d.ownerID == "" && d.trustedChannel == "" {
		return true
	}

	return false
}

func (d *discord) downloadAttachment(url string) ([]byte, string, error) {
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
