package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"VLX_AudioBridge/internal/config"
	"VLX_AudioBridge/internal/overlay"
	"VLX_AudioBridge/internal/stream"
)

type Bot struct {
	Session         *discordgo.Session
	Config          *config.Config
	StreamManager   *stream.Manager
	VoiceConnection *discordgo.VoiceConnection
	StopCaptureChan chan struct{}
	OwnerID         string // Application Owner ID for command authorization
}

// New initializes a new Bot instance.
func New(cfg *config.Config, sm *stream.Manager) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	// Set required intents: GuildMessages for commands, GuildVoiceStates for channel detection
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates

	b := &Bot{
		Session:       dg,
		Config:        cfg,
		StreamManager: sm,
	}

	dg.AddHandler(b.onReady)
	dg.AddHandler(b.onMessageCreate)

	return b, nil
}

// Open establishes the websocket connection.
func (b *Bot) Open() error {
	return b.Session.Open()
}

// Close terminates the connection and cleans up resources.
func (b *Bot) Close() {
	if b.VoiceConnection != nil {
		// Context required for disconnect in the current library fork
		b.VoiceConnection.Disconnect(context.Background())
	}
	b.Session.Close()
}

// isOwner verifies if the user is authorized to execute commands.
func (b *Bot) isOwner(userID string) bool {
	// 1. Check Application Owner
	if b.OwnerID != "" && userID == b.OwnerID {
		return true
	}
	// 2. Check Whitelist (ExcludedUsers)
	for _, allowedID := range b.Config.Streaming.ExcludedUsers {
		if userID == allowedID {
			return true
		}
	}
	return false
}

// --- Event Handlers ---

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("[Bot] Session started. Logged in as: %s#%s", s.State.User.Username, s.State.User.Discriminator)
	
	// Retrieve Application Info to automatically set OwnerID
	app, err := s.Application("@me")
	if err != nil {
		log.Printf("[Bot] Warning: Failed to fetch application info: %v", err)
	} else if app.Owner != nil {
		b.OwnerID = app.Owner.ID
		log.Printf("[Bot] Owner detected: %s. Commands restricted to this user.", b.OwnerID)
	}
}

func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bots and messages without prefix
	if m.Author.Bot || !strings.HasPrefix(m.Content, b.Config.Discord.Prefix) {
		return
	}

	// Strict Owner Check
	if !b.isOwner(m.Author.ID) {
		return
	}

	// Command Parsing
	rawContent := strings.TrimPrefix(m.Content, b.Config.Discord.Prefix)
	parts := strings.Fields(rawContent)
	
	if len(parts) == 0 {
		return
	}
	
	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "join":
		b.handleJoin(s, m, args)
	case "leave":
		b.handleLeave(s, m)
	case "shutdown":
		b.handleShutdown(s, m)
	}
}

// --- Command Implementation ---

func (b *Bot) handleJoin(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	var channelID string

	// 1. Manual Argument (Priority)
	if len(args) > 0 {
		channelID = args[0]
	} else {
		// 2. Auto-detect user channel
		vs, err := s.State.VoiceState(m.GuildID, m.Author.ID)
		if err == nil {
			channelID = vs.ChannelID
		}
	}

	if channelID == "" {
		s.ChannelMessageSend(m.ChannelID, "Error: Missing Channel ID. Usage: `vlx.join <ChannelID>`")
		return
	}

	// Join Voice Channel. 
	// Mute/Deaf must be false to allow bidirectional audio (Overlay injection + SRT capture).
	// Context is required for the Join call in this library version.
	vc, err := s.ChannelVoiceJoin(context.Background(), m.GuildID, channelID, false, false)
	if err != nil {
		log.Printf("[Bot] Voice connection failed: %v", err)
		s.ChannelMessageSend(m.ChannelID, "Error: Failed to join voice channel.")
		return
	}
	b.VoiceConnection = vc

	// 1. Start SRT Packet Capture (Discord -> MediaMTX)
	go func() {
		log.Println("[Bot] Starting packet capture loop.")
		for p := range vc.OpusRecv {
			if b.StreamManager != nil {
				b.StreamManager.HandlePacket(p)
			}
		}
	}()

	if b.StreamManager != nil {
		if err := b.StreamManager.Start(); err != nil {
			log.Printf("[Bot] Stream Manager start failed: %v", err)
		}
	}

	// 2. Start Overlay Audio Injection (Pipewire -> Discord)
	b.StopCaptureChan = make(chan struct{})
	go func() {
		if err := overlay.CaptureAndStream(vc, b.StopCaptureChan); err != nil {
			log.Printf("[Bot] Overlay capture failed: %v", err)
		}
	}()

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Connected to %s. Audio bridging active.", channelID))
}

func (b *Bot) handleLeave(s *discordgo.Session, m *discordgo.MessageCreate) {
	if b.VoiceConnection == nil {
		return
	}

	// Stop Overlay Capture
	if b.StopCaptureChan != nil {
		close(b.StopCaptureChan)
	}
	// Stop SRT Stream
	if b.StreamManager != nil {
		b.StreamManager.Stop()
	}

	// Disconnect Voice
	b.VoiceConnection.Disconnect(context.Background())
	b.VoiceConnection = nil
	
	s.ChannelMessageSend(m.ChannelID, "Disconnected.")
	log.Println("[Bot] Voice connection closed.")
}

func (b *Bot) handleShutdown(s *discordgo.Session, m *discordgo.MessageCreate) {
	s.ChannelMessageSend(m.ChannelID, "System shutting down...")
	b.handleLeave(s, m)
}
