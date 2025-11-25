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

// SetupPipewire check if audio service is loaded, then setup the Virtual Sink
func SetupPipewire() error {
	// 1. Verify PulseAudio/Pipewire
	cmd := exec.Command("pactl", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("[ERR]: Audio server not available (pactl info failed): %w. Is pipewire/pulse active?", err)
	}

	// 2. Check if Virtual sink exists
	checkCmd := exec.Command("pactl", "list", "sinks", "short")
	output, err := checkCmd.Output()
	if err != nil {
		return fmt.Errorf("[ERR]: Cannot verify sink list: %w", err)
	}

	if strings.Contains(string(output), SinkName) {
		fmt.Println("[INFO] [System]: Virtual Sink already set.")
		return nil
	}

	// 3. If none, create a Null Sink (Virtal output) -> setting "Monitor" -> browsers sounds play here
	createCmd := exec.Command("pactl", "load-module", "module-null-sink",
		fmt.Sprintf("sink_name=%s", SinkName),
		fmt.Sprintf("sink_properties=device.description=%s", SinkDescription),
	)

	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("[ERR]: Cannot create the Virtual Sink: %w", err)
	}

	fmt.Println("[[INFO] [System]: Virtual Sink ready:", SinkName)
	return nil
}
