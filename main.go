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
	// 1. Parse command line arguments
	configPath := flag.String("config", "AudioBridge.yaml", "Path to configuration file")
	flag.Parse()

	// 2. Load Configuration
	log.Println("[INFO]: Loading configuration...")
	if err := config.LoadConfig(*configPath); err != nil {
		log.Fatalf("[ERR]: Critical error loading config: %v", err)
	}

	// 3. System Audio Setup (Pipewire/PulseAudio)
	log.Println("[INFO]: Verifying audio system status...")
	if err := system.SetupPipewire(); err != nil {
		log.Fatalf("[ERR]: Pipewire setup failed: %v", err)
	}

	// 4. Initialize Overlay Manager (Headless Browsers)
	log.Println("[INFO]: Initializing overlay manager...")
	if err := overlay.Start(config.Cfg.Overlays.URLs); err != nil {
		log.Fatalf("[ERR]: Failed to start overlays: %v", err)
	}
	// Ensure browsers are terminated on exit
	defer overlay.Stop()

	// 5. Initialize Streaming Manager
	streamManager := stream.NewManager(config.Cfg.Streaming)

	// 6. Graceful Shutdown Handler (Initialized early to pass to Bot)
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	// 7. Initialize and Launch Discord Bot
	log.Println("[INFO]: Launching Discord bot...")
	discordBot, err := bot.New(config.Cfg, streamManager, sc)
	if err != nil {
		log.Fatalf("[ERR]: Failed to create Discord bot instance: %v", err)
	}

	if err := discordBot.Open(); err != nil {
		log.Fatalf("[ERR]: Failed to establish Discord connection: %v", err)
	}
	defer discordBot.Close()

	log.Println("[INFO]: VLX_AudioBridge is running. Press CTRL+C to exit.")

	// 8. Wait for shutdown signal (from OS or Bot)
	<-sc

	log.Println("[INFO]: Shutdown signal received. Exiting...")
}
