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
	FramesPerBuffer = 960 // 20ms at 48kHz
)

// CaptureAndStream captures audio from the Virtual Sink Monitor and sends it to Discord.
// This function blocks until stopChan is closed.
func CaptureAndStream(vc *discordgo.VoiceConnection, stopChan <-chan struct{}) error {
	log.Println("[AudioCapture] Initializing PortAudio...")
	if err := portaudio.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize PortAudio: %w", err)
	}
	defer portaudio.Terminate()

	// 1. Locate the Monitor device for the Virtual Sink
	devices, err := portaudio.Devices()
	if err != nil {
		return fmt.Errorf("failed to list audio devices: %w", err)
	}

	var inputDevice *portaudio.DeviceInfo
	targetDeviceName := "VLX_VirtualSink.monitor"

	for _, device := range devices {
		if strings.Contains(device.Name, targetDeviceName) {
			inputDevice = device
			break
		}
	}

	if inputDevice == nil {
		return fmt.Errorf("virtual sink monitor device '%s' not found", targetDeviceName)
	}

	log.Printf("[AudioCapture] Capturing from device: %s", inputDevice.Name)

	// 2. Initialize Opus Encoder (optimized for audio)
	encoder, err := opus.NewEncoder(SampleRate, Channels, opus.AppAudio)
	if err != nil {
		return fmt.Errorf("failed to create Opus encoder: %w", err)
	}
	encoder.SetBitrate(96000)

	// 3. Initialize Audio Stream in Blocking Mode
	inputBuffer := make([]float32, FramesPerBuffer*Channels)
	stream, err := portaudio.OpenStream(portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   inputDevice,
			Channels: Channels,
		},
		SampleRate:      SampleRate,
		FramesPerBuffer: FramesPerBuffer,
	}, inputBuffer) 

	if err != nil {
		return fmt.Errorf("failed to open PortAudio stream: %w", err)
	}
	if err := stream.Start(); err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}
	defer stream.Close()

	log.Println("[AudioCapture] Stream started. Relaying audio to Discord...")

	opusBuffer := make([]byte, 4000) // Max safe opus packet size

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	// 4. Main Capture Loop
	for {
		select {
		case <-stopChan:
			log.Println("[AudioCapture] Stop signal received. Terminating capture.")
			return nil
		case <-ticker.C:
			// Read raw PCM from PortAudio (Blocking)
			if err := stream.Read(); err != nil {
				log.Printf("[AudioCapture] Error reading audio stream: %v", err)
				continue
			}

			// Encode PCM to Opus
			n, err := encoder.EncodeFloat32(inputBuffer, opusBuffer)
			if err != nil {
				log.Printf("[AudioCapture] Opus encoding error: %v", err)
				continue
			}

			// Send to Discord (Non-blocking)
			select {
			case vc.OpusSend <- opusBuffer[:n]:
				// Packet sent
			default:
				// Channel full, dropping frame to avoid latency buildup
			}
		}
	}
}
