package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

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
	OwnerID         string
	ShutdownChan    chan os.Signal // Channel to signal main process termination
}

// New initializes a new Bot instance.
// Updated to accept the shutdown channel.
func New(cfg *config.Config, sm *stream.Manager, shutdownChan chan os.Signal) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	// Set required intents
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates

	b := &Bot{
		Session:       dg,
		Config:        cfg,
		StreamManager: sm,
		ShutdownChan:  shutdownChan,
	}

	dg.AddHandler(b.onReady)
	dg.AddHandler(b.onMessageCreate)

	return b, nil
}

func (b *Bot) Open() error {
	return b.Session.Open()
}

func (b *Bot) Close() {
	// Graceful shutdown: stop capture goroutines before disconnecting voice
	if b.StopCaptureChan != nil {
		select {
		case <-b.StopCaptureChan:
			// Channel already closed
		default:
			close(b.StopCaptureChan)
		}
		// Allow time for goroutines to terminate
		time.Sleep(100 * time.Millisecond)
	}

	if b.VoiceConnection != nil {
		// Context is required by the ozraru fork
		b.VoiceConnection.Disconnect(context.Background())
	}
	b.Session.Close()
}

func (b *Bot) isOwner(userID string) bool {
	if b.OwnerID != "" && userID == b.OwnerID {
		return true
	}
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
	
	app, err := s.Application("@me")
	if err != nil {
		log.Printf("[Bot] Warning: Failed to fetch application info: %v", err)
	} else if app.Owner != nil {
		b.OwnerID = app.Owner.ID
		log.Printf("[Bot] Owner detected: %s. Commands restricted to this user.", b.OwnerID)
	}
}

func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot || !strings.HasPrefix(m.Content, b.Config.Discord.Prefix) {
		return
	}

	if !b.isOwner(m.Author.ID) {
		return
	}

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

	if len(args) > 0 {
		channelID = args[0]
	} else {
		vs, err := s.State.VoiceState(m.GuildID, m.Author.ID)
		if err == nil {
			channelID = vs.ChannelID
		}
	}

	if channelID == "" {
		s.ChannelMessageSend(m.ChannelID, "Error: Missing Channel ID.")
		return
	}

	// Join Voice Channel using context (required by ozraru fork)
	vc, err := s.ChannelVoiceJoin(context.Background(), m.GuildID, channelID, false, false)
	if err != nil {
		log.Printf("[Bot] Voice connection failed: %v", err)
		s.ChannelMessageSend(m.ChannelID, "Error: Failed to join voice channel.")
		return
	}
	b.VoiceConnection = vc

	s.ChannelMessageSend(m.ChannelID, "Connected. Stabilizing voice uplink...")

	// --- Connection Stabilization Logic ---
	
	// 1. Wait for UDP Handshake completion
	time.Sleep(1 * time.Second)

	// 2. Set Speaking status to allow UDP ingress
	if err := vc.Speaking(true); err != nil {
		log.Printf("[Bot] Warning: Failed to set speaking status: %v", err)
	}

	// 3. UDP Hole Punching: Send silence frames to establish NAT traversal
	silenceFrame := []byte{0xF8, 0xFF, 0xFE} // Opus silence frame
	for i := 0; i < 5; i++ {
		if b.VoiceConnection != nil {
			b.VoiceConnection.OpusSend <- silenceFrame
		}
		time.Sleep(20 * time.Millisecond)
	}

	// --- Start Subsystems ---

	// Start Egress Capture (Discord -> SRT)
	go func() {
		log.Println("[Bot] Starting packet capture loop.")
		for p := range vc.OpusRecv {
			if b.StreamManager != nil {
				b.StreamManager.HandlePacket(p)
			}
		}
		log.Println("[Bot] Packet capture loop terminated.")
	}()

	if b.StreamManager != nil {
		if err := b.StreamManager.Start(); err != nil {
			log.Printf("[Bot] Error starting StreamManager: %v", err)
		}
	}

	// Start Ingress Injection (Overlay -> Discord)
	b.StopCaptureChan = make(chan struct{})
	go func() {
		if err := overlay.CaptureAndStream(vc, b.StopCaptureChan); err != nil {
			log.Printf("[Bot] Error in Overlay capture: %v", err)
		}
	}()

	s.ChannelMessageSend(m.ChannelID, "Audio Bridge Active.")
}

func (b *Bot) handleLeave(s *discordgo.Session, m *discordgo.MessageCreate) {
	if b.VoiceConnection == nil {
		return
	}

	// Stop Overlay Capture
	if b.StopCaptureChan != nil {
		close(b.StopCaptureChan)
		b.StopCaptureChan = nil
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

	// Send termination signal to main process to trigger graceful shutdown
	if b.ShutdownChan != nil {
		log.Println("[Bot] Sending shutdown signal...")
		b.ShutdownChan <- syscall.SIGTERM
	}
}
