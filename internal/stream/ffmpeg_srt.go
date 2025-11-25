package stream

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/viruslox/VLX_AudioBridge/internal/config"
)

type FFmpegProcess struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	isRunning bool
}

func NewFFmpegProcess(cfg config.StreamingConfig) (*FFmpegProcess, error) {
	// Buildinf FFmpeg command
	// -f s16le: PCM Signed 16-bit Little Endian (Discord standard decoded)
	// -ar 48000: Sample rate 48kHz
	// -ac 2: Stereo chan
	// -i pipe:0: getting from Stdin
	args := []string{
		"-re",
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "2",
		"-i", "pipe:0",
		"-c:a", "aac",
		"-b:a", cfg.Bitrate,
		"-f", "mpegts",
		cfg.DestinationURL,
	}

	log.Printf("[INFO] [Stream]: FFmpeg command: ffmpeg %s", strings.Join(args, " "))

	cmd := exec.Command("ffmpeg", args...)

	// Colleghiamo stderr a stdout del bot per debuggare ffmpeg se serve
	// cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("errore pipe stdin ffmpeg: %w", err)
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
	// Monitora uscita prematura
	go func() {
		f.cmd.Wait()
		f.isRunning = false
		log.Println("[Stream] Processo FFmpeg terminato.")
	}()
	return nil
}

func (f *FFmpegProcess) Write(pcmData []byte) (int, error) {
	if !f.isRunning {
		return 0, fmt.Errorf("ffmpeg non in esecuzione")
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
