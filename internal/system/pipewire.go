package system

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	SinkName        = "VLX_VirtualSink"
	SinkDescription = "VLX_Overlay_Audio"
)

// SetupPipewire verifies the audio service status and configures the Virtual Sink.
func SetupPipewire() error {
	// 1. Verify PulseAudio/Pipewire availability
	cmd := exec.Command("pactl", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("[ERR]: Audio server unreachable (pactl info failed): %w. Is pipewire-pulse active?", err)
	}

	// 2. Check if Virtual Sink already exists
	checkCmd := exec.Command("pactl", "list", "sinks", "short")
	output, err := checkCmd.Output()
	if err != nil {
		return fmt.Errorf("[ERR]: Failed to list sinks: %w", err)
	}

	if strings.Contains(string(output), SinkName) {
		fmt.Println("[INFO] [System]: Virtual Sink already configured.")
		return nil
	}

	// 3. Create Null Sink if missing.
	// This sets up a virtual output device with a "Monitor" source where browser audio will be routed.
	createCmd := exec.Command("pactl", "load-module", "module-null-sink",
		fmt.Sprintf("sink_name=%s", SinkName),
		fmt.Sprintf("sink_properties=device.description=%s", SinkDescription),
	)

	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("[ERR]: Failed to create Virtual Sink: %w", err)
	}

	fmt.Println("[INFO] [System]: Virtual Sink ready:", SinkName)
	return nil
}
