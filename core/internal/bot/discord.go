package bot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/bowerhall/sheldon/internal/agent"
	"github.com/bowerhall/sheldon/internal/logger"
)

type discord struct {
	session *discordgo.Session
	agent   *agent.Agent
	ctx     context.Context
}

func newDiscord(token string, agent *agent.Agent) (Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	d := &discord{
		session: session,
		agent:   agent,
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

func (d *discord) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	sessionID := fmt.Sprintf("discord:%s", m.ChannelID)
	logger.Info("message received", "from", m.Author.Username, "text", truncate(m.Content, 50))

	response, err := d.agent.Process(d.ctx, sessionID, m.Content)
	if err != nil {
		logger.Error("agent failed", "error", err)
		response = "Something went wrong."
	}

	if _, err := s.ChannelMessageSendReply(m.ChannelID, response, m.Reference()); err != nil {
		logger.Error("discord reply failed", "error", err)
	} else {
		logger.Info("reply sent", "chars", len(response))
	}
}
