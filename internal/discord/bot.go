package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

// Bot wraps a discordgo session with DM-only filtering.
type Bot struct {
	session       *discordgo.Session
	allowedUserID string
	onMessage     func(userID, content, msgID string)
}

// NewBot creates a Discord bot that only accepts DMs from the allowed user.
func NewBot(token, allowedUserID string) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}

	dg.Identify.Intents = discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	bot := &Bot{
		session:       dg,
		allowedUserID: allowedUserID,
	}

	dg.AddHandler(bot.handleMessage)
	return bot, nil
}

// OnMessage sets the handler for incoming DM messages.
func (b *Bot) OnMessage(fn func(userID, content, msgID string)) {
	b.onMessage = fn
}

// Start opens the Discord websocket connection.
func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("open discord: %w", err)
	}
	log.Printf("discord bot connected")
	return nil
}

// Stop closes the Discord connection.
func (b *Bot) Stop() error {
	return b.session.Close()
}

// SendDM sends a message to the allowed user's DM channel.
func (b *Bot) SendDM(content string) error {
	channel, err := b.session.UserChannelCreate(b.allowedUserID)
	if err != nil {
		return fmt.Errorf("create DM channel: %w", err)
	}

	chunks := SplitMessage(content, 2000)
	for _, chunk := range chunks {
		if _, err := b.session.ChannelMessageSend(channel.ID, chunk); err != nil {
			return fmt.Errorf("send message: %w", err)
		}
	}
	return nil
}

// SendTyping sends a typing indicator to the user's DM channel.
func (b *Bot) SendTyping() {
	channel, err := b.session.UserChannelCreate(b.allowedUserID)
	if err != nil {
		return
	}
	_ = b.session.ChannelTyping(channel.ID)
}

func (b *Bot) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	if m.Author.ID != b.allowedUserID {
		log.Printf("discord: ignoring message from %s", m.Author.ID)
		return
	}
	if m.GuildID != "" {
		return
	}
	if b.onMessage != nil {
		b.onMessage(m.Author.ID, m.Content, m.Message.ID)
	}
}
