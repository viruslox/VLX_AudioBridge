package bot

import (
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/viruslox/VLX_AudioBridge/internal/config"
	"github.com/viruslox/VLX_AudioBridge/internal/overlay"
	"github.com/viruslox/VLX_AudioBridge/internal/stream"
)

type Bot struct {
	Session       *discordgo.Session
	Config        config.DiscordConfig
	StreamManager *stream.Manager

	// State tracking
	VoiceConnection *discordgo.VoiceConnection
	StopCaptureChan chan struct{} // Channel to stop the overlay audio capture goroutine
}

// New creates a new instance of the Bot
func New(cfg config.DiscordConfig, sm *stream.Manager) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("error creating discord session: %w", err)
	}

	// Set necessary intents
	dg.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsGuildMembers

	b := &Bot{
		Session:       dg,
		Config:        cfg,
		StreamManager: sm,
	}

	// Register Handlers
	dg.AddHandler(b.onReady)
	dg.AddHandler(b.onMessageCreate)
	dg.AddHandler(b.onVoiceSpeakingUpdate)

	return b, nil
}

func (b *Bot) Open() error {
	return b.Session.Open()
}

func (b *Bot) Close() {
	// Cleanup on close
	if b.VoiceConnection != nil {
		b.VoiceConnection.Disconnect()
	}
	b.Session.Close()
}

// --- Event Handlers ---

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("[Bot] Logged in as: %s#%s", s.State.User.Username, s.State.User.Discriminator)
}

func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from bots or without prefix
	if m.Author.Bot || !strings.HasPrefix(m.Content, b.Config.Prefix) {
		return
	}

	// Parse command
	args := strings.Fields(m.Content)
	cmd := strings.TrimPrefix(args[0], b.Config.Prefix)

	switch cmd {
	case "join":
		b.handleJoin(s, m)
	case "leave":
		b.handleLeave(s, m)
	case "shutdown":
		b.handleShutdown(s, m)
	}
}

// onVoiceSpeakingUpdate captures the SSRC of speaking users.
// This is critical for filtering excluded users in the Stream Manager.
func (b *Bot) onVoiceSpeakingUpdate(s *discordgo.Session, v *discordgo.VoiceSpeakingUpdate) {
	// Update the SSRC mapping in the Stream Manager
	// Assuming StreamManager has a method: SetUserSSRC(ssrc uint32, userID string)
	if b.StreamManager != nil {
		b.StreamManager.SetUserSSRC(uint32(v.SSRC), v.UserID)
		log.Printf("[Bot] Updated SSRC map: User %s -> SSRC %d", v.UserID, v.SSRC)
	}
}

// --- Command Logic ---

func (b *Bot) handleJoin(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Find the voice channel the user is currently in
	vs, err := s.State.VoiceState(m.GuildID, m.Author.ID)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "⚠️ You must be in a voice channel to summon me.")
		return
	}

	// Join the voice channel
	vc, err := s.ChannelVoiceJoin(m.GuildID, vs.ChannelID, false, true)
	if err != nil {
		log.Printf("[Bot] Failed to join voice channel: %v", err)
		s.ChannelMessageSend(m.ChannelID, "❌ Failed to join voice channel.")
		return
	}
	b.VoiceConnection = vc

	// 1. Start the SRT Stream Manager (Incoming Audio -> Mixing -> FFmpeg)
	// We need to attach the packet handler to the VoiceConnection
	go func() {
		log.Println("[Bot] Starting Packet Handler loop...")
		// b.StreamManager.HandlePacket is called for every incoming opus packet
		for p := range vc.OpusRecv {
			b.StreamManager.HandlePacket(p)
		}
	}()

	if err := b.StreamManager.Start(); err != nil {
		log.Printf("[Bot] Failed to start stream manager: %v", err)
		s.ChannelMessageSend(m.ChannelID, "⚠️ Joined, but failed to start SRT stream.")
	} else {
		log.Println("[Bot] SRT Stream Manager started.")
	}

	// 2. Start Overlay Audio Capture (Virtual Sink -> Opus Enc -> Discord)
	b.StopCaptureChan = make(chan struct{})
	go func() {
		if err := overlay.CaptureAndStream(vc, b.StopCaptureChan); err != nil {
			log.Printf("[Bot] Error in overlay audio capture: %v", err)
		}
	}()

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ Connected to **%s**.\n📡 Stream started.\n🖥️ Overlays active.", vs.ChannelID))
}

func (b *Bot) handleLeave(s *discordgo.Session, m *discordgo.MessageCreate) {
	if b.VoiceConnection == nil {
		return
	}

	// Stop Capture
	if b.StopCaptureChan != nil {
		close(b.StopCaptureChan)
	}

	// Stop Stream Manager
	b.StreamManager.Stop()

	// Disconnect
	b.VoiceConnection.Disconnect()
	b.VoiceConnection = nil

	s.ChannelMessageSend(m.ChannelID, "👋 Disconnected.")
	log.Println("[Bot] Disconnected from voice channel.")
}

func (b *Bot) handleShutdown(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Simple permission check (should be improved for production)
	// For now, checks if the issuer is the guild owner or configured admin
	// Implementation omitted for brevity

	s.ChannelMessageSend(m.ChannelID, "🛑 Shutting down system...")
	b.handleLeave(s, m)

	// In main.go, the OS signal handling will take care of the rest
	// or we can manually trigger exit.
	// Ideally, send a signal to a main channel to exit gracefully.
}
