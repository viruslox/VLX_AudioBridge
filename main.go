package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"VLX_AudioBridge/internal/bot"
	"VLX_AudioBridge/internal/config"
	"VLX_AudioBridge/internal/overlay"
	"VLX_AudioBridge/internal/stream"
	"VLX_AudioBridge/internal/system"
)

func main() {
	// 1. Command line parsing (for additional params)
	configPath := flag.String("config", "AudioBridge.yaml", "Conf file path")
	flag.Parse()

	// 2. Load config
	log.Println("[INFO]: Loading config")
	if err := config.LoadConfig(*configPath); err != nil {
		log.Fatalf("[ERR]: Critical error loading config: %v", err)
	}

	// 3. System setup(Pipewire Check & Virtual Sink)
	log.Println("[INFO]: Checking pipewire status")
	if err := system.SetupPipewire(); err != nil {
		log.Fatalf("[ERR]: Pipewire setup error: %v", err)
	}

	// 4. Overlay Module (Browser Headless)
	log.Println("[INFO]: Loading overlay manager")
	if err := overlay.Start(config.Cfg.Overlays.URLs); err != nil {
		log.Fatalf("[ERR]: Error loading overlay: %v", err)
	}
	// Browser closure on exit
	defer overlay.Stop()

	// 5. Launch streaming (FFmpeg SRT)
	streamManager := stream.NewManager(config.Cfg.Streaming)

	// 6. Discord bot
	log.Println("[INFO]: Launching Discord bot")
	discordBot, err := bot.New(config.Cfg.Discord, streamManager)
	if err != nil {
		log.Fatalf("[ERR]: Failed connecting Discord bot: %v", err)
	}

	if err := discordBot.Open(); err != nil {
		log.Fatalf("[ERR]: Failed connecting to Discord: %v", err)
	}
	defer discordBot.Close()

	log.Println("[INFO]: VLX_AudioBridge is ON! Press CTRL+C to kill it.")

	// 7. Waiting closure call (Graceful Shutdown)
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("[INFO]: Received closure signal, shutting down")
}
