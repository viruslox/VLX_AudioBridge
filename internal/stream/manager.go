package stream

import (
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/hraban/opus"
	"VLX_AudioBridge/internal/config"
)

type Manager struct {
	config        config.StreamingConfig
	ffmpeg        *FFmpegProcess
	mixer         *Mixer
	opusDecoders  map[uint32]*opus.Decoder // One decoder per user (SSRC)
	excludedUsers map[string]bool
	stopChan      chan struct{}
}

func NewManager(cfg config.StreamingConfig) *Manager {
	exMap := make(map[string]bool)
	for _, id := range cfg.ExcludedUsers {
		exMap[id] = true
	}

	return &Manager{
		config:        cfg,
		mixer:         NewMixer(),
		opusDecoders:  make(map[uint32]*opus.Decoder),
		excludedUsers: exMap,
		stopChan:      make(chan struct{}),
	}
}

// Start initiates the streaming process (FFmpeg + Mixer).
func (m *Manager) Start() error {
	var err error
	m.ffmpeg, err = NewFFmpegProcess(m.config)
	if err != nil {
		return err
	}

	if err := m.ffmpeg.Start(); err != nil {
		return err
	}

	// Start the Mixer loop
	go m.mixer.StartMixing(m.stopChan)

	// Start the bridge loop: Mixer -> FFmpeg
	go func() {
		for {
			select {
			case data := <-m.mixer.mixedOut:
				if _, err := m.ffmpeg.Write(data); err != nil {
					log.Println("[Stream] Error writing to FFmpeg pipe:", err)
					return
				}
			case <-m.stopChan:
				return
			}
		}
	}()

	return nil
}

// Stop halts all streaming operations.
func (m *Manager) Stop() {
	close(m.stopChan)
	if m.ffmpeg != nil {
		m.ffmpeg.Stop()
	}
}

// HandlePacket is invoked by the Bot upon receiving an Opus audio packet from Discord.
func (m *Manager) HandlePacket(p *discordgo.Packet) {
	// TODO: Verify user against excludedUsers map using SSRC->UserID mapping.
	// DiscordGo does not provide SSRC in the packet directly mapped to UserID.
	// This mapping is maintained in the Bot struct via OnVoiceSpeakingUpdate.
	// For now, we process all incoming packets.

	// 1. Get or create Opus Decoder for this SSRC
	decoder, exists := m.opusDecoders[p.SSRC]
	if !exists {
		var err error
		// Initialize decoder: 48kHz, 2 channels (Stereo required for mixing)
		decoder, err = opus.NewDecoder(48000, 2)
		if err != nil {
			log.Println("[Stream] Error creating Opus decoder:", err)
			return
		}
		m.opusDecoders[p.SSRC] = decoder
	}

	// 2. Decode Opus -> PCM
	// Buffer size for 20ms at 48kHz stereo (960 samples * 2 channels = 1920 int16)
	pcmBuffer := make([]int16, 1920)
	n, err := decoder.Decode(p.Opus, pcmBuffer)
	if err != nil {
		// Drop packet on decode error or packet loss
		return
	}

	// 3. Send decoded PCM to Mixer
	m.mixer.AddFrame(p.SSRC, pcmBuffer[:n*2])
}

// SetUserSSRC updates the mapping between an SSRC and a UserID.
// This is thread-safe and required for exclusion logic.
func (m *Manager) SetUserSSRC(ssrc uint32, userID string) {
    // Implementation pending:
    // m.mutex.Lock()
    // m.ssrcMap[ssrc] = userID
    // m.mutex.Unlock()
}
