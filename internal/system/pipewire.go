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

// SetupPipewire checks audio server status and configures the virtual sink/source.
func SetupPipewire() error {
	// 1. Verify PulseAudio/Pipewire availability
	cmd := exec.Command("pactl", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("audio server unreachable (pactl info failed): %w", err)
	}

	// 2. Check if Virtual Sink exists
	checkCmd := exec.Command("pactl", "list", "sinks", "short")
	output, err := checkCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list sinks: %w", err)
	}

	if !strings.Contains(string(output), SinkName) {
		// 3. Create Null Sink if missing
		createCmd := exec.Command("pactl", "load-module", "module-null-sink",
			fmt.Sprintf("sink_name=%s", SinkName),
			fmt.Sprintf("sink_properties=device.description=%s", SinkDescription),
		)
		if err := createCmd.Run(); err != nil {
			return fmt.Errorf("failed to create Virtual Sink: %w", err)
		}
		fmt.Println("[System] Virtual Sink created:", SinkName)
	} else {
		fmt.Println("[System] Virtual Sink already configured.")
	}

	// 4. Force Default Source
	// This ensures applications connecting to "default" or "pulse" source capture from our sink monitor.
	monitorName := SinkName + ".monitor"
	setDefaultCmd := exec.Command("pactl", "set-default-source", monitorName)
	if err := setDefaultCmd.Run(); err != nil {
		fmt.Printf("[System] Warning: Failed to set default source to %s: %v\n", monitorName, err)
	} else {
		fmt.Printf("[System] Default source set to: %s\n", monitorName)
	}

	return nil
}
