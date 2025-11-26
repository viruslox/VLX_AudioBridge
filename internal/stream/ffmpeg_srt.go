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
	// Build FFmpeg command arguments:
	// -re: Read input at native frame rate (simulates real-time)
	// -f s16le: Input format PCM Signed 16-bit Little Endian
	// -ar 48000: Input sample rate
	// -ac 2: Input channels (Stereo)
	// -i pipe:0: Read input from Stdin
	args := []string{
		"-re",
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "2",
		"-i", "pipe:0",
		"-c:a", "libopus",
		"-b:a", cfg.Bitrate,
		"-f", "mpegts",
		cfg.DestinationURL,
	}

	log.Printf("[INFO] [Stream]: FFmpeg command: ffmpeg %s", strings.Join(args, " "))

	cmd := exec.Command("ffmpeg", args...)

	// Optional: Connect stderr to parent stdout for debugging
	// cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create ffmpeg stdin pipe: %w", err)
	}

	return &FFmpegProcess{
		cmd:   cmd,
		stdin: stdin,
	}, nil
}

func (f *FFmpegProcess) Start() error {
	if f.isRunning {
		return nil
	}
	if err := f.cmd.Start(); err != nil {
		return err
	}
	f.isRunning = true

	// Monitor for premature exit
	go func() {
		f.cmd.Wait()
		f.isRunning = false
		log.Println("[Stream] FFmpeg process terminated.")
	}()
	return nil
}

func (f *FFmpegProcess) Write(pcmData []byte) (int, error) {
	if !f.isRunning {
		return 0, fmt.Errorf("ffmpeg is not running")
	}
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
