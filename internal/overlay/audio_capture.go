package overlay

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gordonklaus/portaudio"
	"github.com/hraban/opus"
)

const (
	SampleRate      = 48000
	Channels        = 2
	FramesPerBuffer = 960 // 20ms audio frame
	BufferSize      = 50  // Ring buffer size (approx 1s) to mitigate jitter
)

// CaptureAndStream handles audio capture from system and streaming to Discord.
func CaptureAndStream(vc *discordgo.VoiceConnection, stopChan <-chan struct{}) error {
	log.Println("[AudioCapture] Initializing PortAudio...")
	if err := portaudio.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize PortAudio: %w", err)
	}
	defer portaudio.Terminate()

	// --- Device Selection Logic ---
	devices, err := portaudio.Devices()
	if err != nil {
		return fmt.Errorf("failed to list audio devices: %w", err)
	}

	var inputDevice *portaudio.DeviceInfo
	targetName := "VLX_VirtualSink"

	for _, device := range devices {
		if device.MaxInputChannels > 0 {
			// Priority 1: Exact match for our sink monitor
			if strings.Contains(device.Name, targetName) {
				inputDevice = device
				break
			}
			// Priority 2: Fallback to generic 'pulse' or 'default' devices
			if inputDevice == nil && (device.Name == "pulse" || device.Name == "default") {
				inputDevice = device
			}
		}
	}

	// Priority 3: Last resort fallback
	if inputDevice == nil && len(devices) > 0 {
		for _, d := range devices {
			if d.MaxInputChannels > 0 {
				inputDevice = d
				break
			}
		}
	}

	if inputDevice == nil {
		return fmt.Errorf("no suitable input device found")
	}
	log.Printf("[AudioCapture] Selected device: %s", inputDevice.Name)

	// --- Encoder Setup ---
	encoder, err := opus.NewEncoder(SampleRate, Channels, opus.AppAudio)
	if err != nil {
		return fmt.Errorf("failed to create Opus encoder: %w", err)
	}
	encoder.SetBitrate(64000) // 64kbps for stability

	// --- Ring Buffer Channel ---
	pcmChan := make(chan []float32, BufferSize)

	// --- PortAudio Stream (Callback Mode) ---
	// Uses callback to decouple audio capture timing from network timing
	stream, err := portaudio.OpenStream(portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   inputDevice,
			Channels: Channels,
		},
		SampleRate:      SampleRate,
		FramesPerBuffer: FramesPerBuffer,
	}, func(in []float32) {
		buf := make([]float32, len(in))
		copy(buf, in)
		
		select {
		case pcmChan <- buf:
		default:
			// Buffer full, dropping packet to maintain real-time stream
		}
	})

	if err != nil {
		return fmt.Errorf("failed to open audio stream: %w", err)
	}
	if err := stream.Start(); err != nil {
		return fmt.Errorf("failed to start audio stream: %w", err)
	}
	defer stream.Close()

	log.Println("[AudioCapture] Streaming active via Jitter Buffer.")

	opusBuffer := make([]byte, 4000)
	silence := make([]float32, FramesPerBuffer*Channels)
	
	// --- Transmission Loop (Fixed 20ms Interval) ---
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			log.Println("[AudioCapture] Stop signal received.")
			return nil
		case <-ticker.C:
			var frame []float32
			
			select {
			case frame = <-pcmChan:
				// Audio data available
			default:
				// Buffer underrun: send silence to keep UDP connection alive
				frame = silence
			}

			n, err := encoder.EncodeFloat32(frame, opusBuffer)
			if err != nil {
				continue
			}

			select {
			case vc.OpusSend <- opusBuffer[:n]:
				// Packet sent
			default:
				// Network congestion, drop packet
			}
		}
	}
}
