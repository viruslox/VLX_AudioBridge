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
		// Note: "-re" flag removed as the Go Mixer already dictates real-time timing.
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "2",
		"-i", "pipe:0",
		"-c:a", "libopus",
		"-b:a", cfg.Bitrate,
		"-f", "mpegts",
		"-flush_packets", "0",
		// FFmpeg low latency flags
		"-fflags", "nobuffer", 
		"-flags", "low_delay",
	}

	// Append pkt_size to SRT destination for stability
	destination := cfg.DestinationURL
	if strings.HasPrefix(destination, "srt://") && !strings.Contains(destination, "pkt_size") {
		 destination += "&pkt_size=1316"
	}
	args = append(args, destination)

	log.Printf("[INFO] [Stream]: FFmpeg command: ffmpeg %s", strings.Join(args, " "))

	// Set to nil for production cleanliness, or os.Stderr for debugging
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = nil 

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create ffmpeg stdin pipe: %w", err)
	}

	return &FFmpegProcess{
		cmd:   cmd,
		stdin: stdin,
	}, nil
}

// ... (Rest of the file remains unchanged as it was already correct) ...
// (Mantieni Start, Write e Stop come sono)
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
