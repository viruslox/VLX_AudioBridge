package overlay

import (
	"log"
	"os"
	"os/exec"
)

var activeBrowsers []*exec.Cmd

// Start launches headless Chromium instances for given URLs.
func Start(urls []string) error {
	for _, url := range urls {
		log.Printf("[Overlay] Launching headless browser for URL: %s", url)

		// NOTE: "chromium" is the standard binary name on most Linux distros.
		// "--autoplay-policy=no-user-gesture-required" is mandatory for audio in headless mode.
		cmd := exec.Command("chromium",
			"--headless",
			"--disable-gpu",
			"--no-sandbox",
			"--remote-debugging-port=9222",
			"--autoplay-policy=no-user-gesture-required",
			"--disable-dev-shm-usage",
			url,
		)

		// Inject PULSE_SINK to route audio to our virtual sink
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
