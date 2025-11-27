package stream

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"

	"VLX_AudioBridge/internal/config"
)

type FFmpegProcess struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	isRunning bool
}

func NewFFmpegProcess(cfg config.StreamingConfig) (*FFmpegProcess, error) {
	args := []string{
		// RIMOSSO: "-re", perché il Mixer Go è già sincronizzato in tempo reale.
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "2",
		"-i", "pipe:0",
		"-c:a", "libopus", // O "aac" se preferisci
		"-b:a", cfg.Bitrate,
		"-f", "mpegts",
		"-flush_packets", "0",
		// Tweaks per bassa latenza FFmpeg
		"-fflags", "nobuffer", 
		"-flags", "low_delay",
	}

	// Aggiungi pkt_size se SRT
	destination := cfg.DestinationURL
	if strings.HasPrefix(destination, "srt://") && !strings.Contains(destination, "pkt_size") {
		 destination += "&pkt_size=1316"
	}
	args = append(args, destination)

	log.Printf("[INFO] [Stream]: FFmpeg command: ffmpeg %s", strings.Join(args, " "))

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = nil // Puoi rimettere os.Stderr se vuoi debuggare, ma nil è più pulito in prod

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create ffmpeg stdin pipe: %w", err)
	}

	return &FFmpegProcess{
		cmd:   cmd,
		stdin: stdin,
	}, nil
}

// ... (Start, Write, Stop rimangono invariati) ...
func (f *FFmpegProcess) Start() error {
	if f.isRunning { return nil }
	if err := f.cmd.Start(); err != nil { return err }
	f.isRunning = true
	go func() {
		f.cmd.Wait()
		f.isRunning = false
		log.Println("[Stream] FFmpeg process terminated.")
	}()
	return nil
}

func (f *FFmpegProcess) Write(pcmData []byte) (int, error) {
	if !f.isRunning { return 0, fmt.Errorf("ffmpeg is not running") }
	return f.stdin.Write(pcmData)
}

func (f *FFmpegProcess) Stop() {
	if f.isRunning {
		f.stdin.Close()
		if f.cmd.Process != nil {
			f.cmd.Process.Kill()
		}
		f.isRunning = false
	}
}
