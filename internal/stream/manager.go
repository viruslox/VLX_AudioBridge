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
	opusDecoders  map[uint32]*opus.Decoder // Un decoder per ogni utente (SSRC)
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

// Start avvia il processo di streaming (FFmpeg + Mixer)
func (m *Manager) Start() error {
	var err error
	m.ffmpeg, err = NewFFmpegProcess(m.config)
	if err != nil {
		return err
	}

	if err := m.ffmpeg.Start(); err != nil {
		return err
	}

	// Avvia il loop del Mixer
	go m.mixer.StartMixing(m.stopChan)

	// Avvia il loop che sposta i dati dal Mixer a FFmpeg
	go func() {
		for {
			select {
			case data := <-m.mixer.mixedOut:
				if _, err := m.ffmpeg.Write(data); err != nil {
					log.Println("[Stream] Errore scrittura FFmpeg:", err)
					return
				}
			case <-m.stopChan:
				return
			}
		}
	}()

	return nil
}

// Stop ferma tutto
func (m *Manager) Stop() {
	close(m.stopChan)
	if m.ffmpeg != nil {
		m.ffmpeg.Stop()
	}
}

// HandlePacket è chiamato dal Bot quando arriva un pacchetto audio da Discord
func (m *Manager) HandlePacket(p *discordgo.Packet) {
	// Nota: qui servirebbe mappare SSRC -> UserID per controllare excludedUsers
	// DiscordGo purtroppo non fornisce un mapping diretto facile dentro OnVoiceStateUpdate per gli SSRC
	// Tuttavia, possiamo implementare una logica nel Bot per mappare UserID <-> SSRC e passarla qui.
	// Per ora assumiamo che p.SSRC sia valido.

	// 1. Ottieni o crea Decoder per questo SSRC
	decoder, exists := m.opusDecoders[p.SSRC]
	if !exists {
		var err error
		// 48kHz, 2 canali (Stereo per il mixing)
		decoder, err = opus.NewDecoder(48000, 2)
		if err != nil {
			log.Println("[Stream] Errore creazione decoder Opus:", err)
			return
		}
		m.opusDecoders[p.SSRC] = decoder
	}

	// 2. Decodifica Opus -> PCM
	// Buffer per 20ms a 48kHz stereo (960 * 2 = 1920 int16)
	pcmBuffer := make([]int16, 1920)
	n, err := decoder.Decode(p.Opus, pcmBuffer)
	if err != nil {
		// Packet loss o errore decode
		return
	}

	// 3. Invia al Mixer
	m.mixer.AddFrame(p.SSRC, pcmBuffer[:n*2]) // n è il numero di sample per canale, slice * canali
}

// SetUserSSRC updates the mapping between an SSRC and a UserID.
// Thread-safe implementation recommended if accessed concurrently.
func (m *Manager) SetUserSSRC(ssrc uint32, userID string) {
    // You might need a mutex here if not already present
    // m.mutex.Lock()
    // defer m.mutex.Unlock()

    // Logic to store mapping, needed for exclusion check in HandlePacket
    // For example, you might need a new map inside Manager struct: ssrcMap map[uint32]string
    // m.ssrcMap[ssrc] = userID
}
