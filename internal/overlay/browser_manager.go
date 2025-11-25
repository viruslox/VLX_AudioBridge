package overlay

import (
	"log"
	"os"
	"os/exec"
)

var activeBrowsers []*exec.Cmd

// StartBrowsers launches headless Chromium instances for each provided URL.
// It sets the PULSE_SINK environment variable to route audio to the Virtual Sink.
func StartBrowsers(urls []string) error {
	// Loop through the URLs provided in the configuration
	for _, url := range urls {
		log.Printf("[Overlay] Launching headless browser for URL: %s", url)

		// Prepare the command to start Chromium/Chrome in headless mode.
		// --remote-debugging-port is required for headless mode in some environments.
		// --no-sandbox is often required when running as root/docker (use with caution).
		cmd := exec.Command("chromium-browser",
			"--headless",
			"--disable-gpu",
			"--no-sandbox", // Necessary if running as root or in certain containerized envs
			"--remote-debugging-port=9222",
			url,
		)

		// Clone the current environment variables
		env := os.Environ()
		// Inject the PULSE_SINK variable to force audio output to our virtual device.
		// This must match the SinkName defined in internal/system/pipewire.go
		env = append(env, "PULSE_SINK=VLX_VirtualSink")
		cmd.Env = env

		// Start the process asynchronously
		if err := cmd.Start(); err != nil {
			log.Printf("[Overlay] Failed to start browser for %s: %v", url, err)
			// We continue trying to launch other URLs even if one fails
			continue
		}

		activeBrowsers = append(activeBrowsers, cmd)
	}
	return nil
}

// StopBrowsers terminates all active browser processes.
func StopBrowsers() {
	log.Println("[Overlay] Stopping all browser instances...")
	for _, cmd := range activeBrowsers {
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				log.Printf("[Overlay] Error killing process: %v", err)
			}
		}
	}
	// Clear the slice
	activeBrowsers = nil
}
