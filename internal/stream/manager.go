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
	opusDecoders  map[uint32]*opus.Decoder
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

func (m *Manager) Start() error {
	var err error
	m.ffmpeg, err = NewFFmpegProcess(m.config)
	if err != nil {
		return err
	}
	if err := m.ffmpeg.Start(); err != nil {
		return err
	}
	go m.mixer.StartMixing(m.stopChan)
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

func (m *Manager) Stop() {
	close(m.stopChan)
	if m.ffmpeg != nil {
		m.ffmpeg.Stop()
	}
}

func (m *Manager) HandlePacket(p *discordgo.Packet) {
	decoder, exists := m.opusDecoders[p.SSRC]
	if !exists {
		var err error
		decoder, err = opus.NewDecoder(48000, 2)
		if err != nil {
			log.Println("[Stream] Error creating Opus decoder:", err)
			return
		}
		m.opusDecoders[p.SSRC] = decoder
	}

	// Buffer size accomodates up to 60ms Opus frames (max Soundboard size)
	// 60ms * 48000Hz = 2880 samples * 2 channels = 5760 int16s
	pcmBuffer := make([]int16, 5760) 
	
	n, err := decoder.Decode(p.Opus, pcmBuffer)
	if err != nil {
		// Log only critical errors, ignore occasional 'corrupted stream' which is expected on UDP
		if err.Error() != "opus: corrupted stream" {
			log.Printf("[Stream] Decode Error for SSRC %d: %v", p.SSRC, err)
		}
		return
	}

	// Pass decoded PCM to mixer
	m.mixer.AddFrame(p.SSRC, pcmBuffer[:n*2])
}

func (m *Manager) SetUserSSRC(ssrc uint32, userID string) {
    // Placeholder for future SSRC-UserID mapping logic
}
