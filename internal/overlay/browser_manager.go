package overlay

import (
	"log"
	"os"
	"os/exec"
)

var activeBrowsers []*exec.Cmd

// Start launches headless Chromium instances for each provided URL.
// It injects the PULSE_SINK environment variable to route audio to the dedicated Virtual Sink.
func Start(urls []string) error {
	for _, url := range urls {
		log.Printf("[Overlay] Launching headless browser for URL: %s", url)

		// --remote-debugging-port is required for headless mode in specific environments.
		// --no-sandbox is often required when running as root or inside containers.
		cmd := exec.Command("chromium-browser",
			"--headless",
			"--disable-gpu",
			"--no-sandbox",
			"--remote-debugging-port=9222",
			url,
		)

		// Clone current environment and inject PULSE_SINK.
		// This must match the SinkName defined in internal/system/pipewire.go.
		env := os.Environ()
		env = append(env, "PULSE_SINK=VLX_VirtualSink")
		cmd.Env = env

		if err := cmd.Start(); err != nil {
			log.Printf("[Overlay] Failed to start browser for %s: %v", url, err)
			continue
		}

		activeBrowsers = append(activeBrowsers, cmd)
	}
	return nil
}

// Stop terminates all active browser processes.
func Stop() {
	log.Println("[Overlay] Stopping all browser instances...")
	for _, cmd := range activeBrowsers {
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				log.Printf("[Overlay] Error killing process: %v", err)
			}
		}
	}
	activeBrowsers = nil
}
