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
	// Ensure PortAudio is terminated when function exits
	defer portaudio.Terminate()

	// 1. Find the Monitor device for our Virtual Sink
	devices, err := portaudio.Devices()
	if err != nil {
		return fmt.Errorf("failed to list audio devices: %w", err)
	}

	var inputDevice *portaudio.DeviceInfo
	targetDeviceName := "VLX_VirtualSink.monitor"

	for _, device := range devices {
		// Log devices for debugging purposes
		// log.Printf("[AudioCapture] Found device: %s", device.Name)
		if strings.Contains(device.Name, targetDeviceName) {
			inputDevice = device
			break
		}
	}

	if inputDevice == nil {
		return fmt.Errorf("virtual sink monitor device '%s' not found", targetDeviceName)
	}

	log.Printf("[AudioCapture] capturing from device: %s", inputDevice.Name)

	// 2. Initialize Opus Encoder
	// ApplicationAudio is optimized for music/high-quality audio
	encoder, err := opus.NewEncoder(SampleRate, Channels, opus.AppAudio)
	if err != nil {
		return fmt.Errorf("failed to create Opus encoder: %w", err)
	}
	// Set bitrate to match config if needed, or default to high quality (e.g., 96kbps)
	encoder.SetBitrate(96000)

	// 3. Open Audio Stream
	// We read float32 samples from PortAudio
	inputBuffer := make([]float32, FramesPerBuffer*Channels)
	stream, err := portaudio.OpenStream(portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   inputDevice,
			Channels: Channels,
		},
		SampleRate:      SampleRate,
		FramesPerBuffer: FramesPerBuffer,
	}, func(in []float32) {
		// Copy input callback data to our buffer
		copy(inputBuffer, in)
	})
	if err != nil {
		return fmt.Errorf("failed to open PortAudio stream: %w", err)
	}

	if err := stream.Start(); err != nil {
		return fmt.Errorf("failed to start audio stream: %w", err)
	}
	defer stream.Close()

	log.Println("[AudioCapture] Stream started. Sending audio to Discord...")

	// 4. Processing Loop
	// Since PortAudio uses a callback (in the Go wrapper handled differently or via blocking Read),
	// but the wrapper we used above has a callback signature.
	// Wait! The wrapper above uses a callback signature in OpenStream.
	// However, to sync with Discord's 20ms requirement, blocking Read is often easier
	// than managing channels in a callback. Let's switch to Blocking Read for simplicity and stability.

	// Re-opening stream in Blocking Read mode (no callback function provided)
	stream.Close()
	stream, err = portaudio.OpenStream(portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   inputDevice,
			Channels: Channels,
		},
		SampleRate:      SampleRate,
		FramesPerBuffer: FramesPerBuffer,
	}, inputBuffer) // Passing buffer pointer tells wrapper to use blocking Read/Write

	if err != nil {
		return fmt.Errorf("failed to open blocking stream: %w", err)
	}
	if err := stream.Start(); err != nil {
		return err
	}

	opusBuffer := make([]byte, 4000) // Max opus packet size

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			log.Println("[AudioCapture] Stop signal received. Shutting down capture.")
			return nil
		case <-ticker.C:
			// Read raw PCM from PortAudio
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

			// Send to Discord
			// vc.OpusSend is a buffered channel provided by DiscordGo
			select {
			case vc.OpusSend <- opusBuffer[:n]:
				// Sent successfully
			default:
				// Channel full, drop frame to avoid lag accumulation
				// log.Println("[AudioCapture] Discord OpusSend channel full, dropping frame")
			}
		}
	}
}
