package discord

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/mattw/ai-lab/internal/claude"
	"github.com/mattw/ai-lab/internal/eventbus"
)

// Bridge connects a Discord bot to a Claude session, forwarding messages
// in both directions and persisting them to the database.
type Bridge struct {
	bot     *Bot
	session *claude.SessionManager
	db      *sql.DB
	bus     *eventbus.EventBus
}

// NewBridge creates a Bridge that wires together Discord and Claude.
func NewBridge(bot *Bot, session *claude.SessionManager, db *sql.DB, bus *eventbus.EventBus) *Bridge {
	return &Bridge{
		bot:     bot,
		session: session,
		db:      db,
		bus:     bus,
	}
}

// Start registers the incoming message handler and launches a goroutine
// to relay Claude's responses back to Discord.
func (br *Bridge) Start() {
	br.bot.OnMessage(br.handleIncoming)
	go br.handleOutgoing()
}

// handleIncoming processes a DM received from the allowed Discord user.
func (br *Bridge) handleIncoming(userID, content, msgID string) {
	log.Printf("discord bridge: received message from %s: %s", userID, truncate(content, 80))

	br.storeMessage("user", content, msgID)

	br.bot.SendTyping()

	br.bus.Publish(eventbus.Event{
		Source:    "discord",
		Type:     "message.received",
		Summary:  truncate(content, 120),
		SessionID: br.session.SessionID(),
	})

	if err := br.session.Send(content); err != nil {
		log.Printf("discord bridge: send to claude: %v", err)
	}
}

// handleOutgoing reads events from the Claude session and relays assistant
// responses back to the Discord user.
func (br *Bridge) handleOutgoing() {
	for evt := range br.session.Events() {
		switch evt.Type {
		case "assistant":
			aEvt, err := claude.ParseAssistantEvent(evt.Raw)
			if err != nil {
				log.Printf("discord bridge: parse assistant event: %v", err)
				continue
			}
			text := aEvt.FullText()
			if text == "" {
				continue
			}

			if err := br.bot.SendDM(text); err != nil {
				log.Printf("discord bridge: send DM: %v", err)
				continue
			}

			br.storeMessage("assistant", text, "")

			br.bus.Publish(eventbus.Event{
				Source:    "discord",
				Type:     "message.sent",
				Summary:  truncate(text, 120),
				SessionID: br.session.SessionID(),
			})

		case "result":
			rEvt, err := claude.ParseResultEvent(evt.Raw)
			if err != nil {
				log.Printf("discord bridge: parse result event: %v", err)
				continue
			}
			log.Printf("discord bridge: result - cost=$%.4f duration=%s turns=%d",
				rEvt.TotalCostUSD,
				time.Duration(rEvt.DurationMS)*time.Millisecond,
				rEvt.NumTurns,
			)
		}
	}
}

// storeMessage inserts a message record into the database.
func (br *Bridge) storeMessage(role, content, discordMsgID string) {
	_, err := br.db.Exec(
		`INSERT INTO messages (session_id, role, content, discord_msg_id) VALUES (?, ?, ?, ?)`,
		br.session.SessionID(), role, content, discordMsgID,
	)
	if err != nil {
		log.Printf("discord bridge: store message: %v", err)
	}
}

// truncate shortens s to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return fmt.Sprintf("%s...", s[:maxLen-3])
}
