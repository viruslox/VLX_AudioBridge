package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the structure of AudioBridge.yaml
type Config struct {
	Discord   DiscordConfig   `yaml:"discord"`
	Streaming StreamingConfig `yaml:"streaming"`
	Overlays  OverlaysConfig  `yaml:"overlays"`
}

type DiscordConfig struct {
	Token   string `yaml:"token"`
	Prefix  string `yaml:"prefix"`
	GuildID string `yaml:"guild_id"`
}

type StreamingConfig struct {
	DestinationURL string   `yaml:"destination_url"`
	Bitrate        string   `yaml:"bitrate"`
	ExcludedUsers  []string `yaml:"excluded_users"`
}

type OverlaysConfig struct {
	URLs []string `yaml:"urls"`
}

// Global config variable
var Cfg *Config

func LoadConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("[ERR]: Cannot open config file: %w", err)
	}
	defer f.Close()

	var cfg Config
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&cfg); err != nil {
		return fmt.Errorf("[ERR]: YAML parsing error: %w", err)
	}

	if len(cfg.Streaming.ExcludedUsers) > 2 {
		return fmt.Errorf("[ERR]: Too many excluded users in config (max 2)")
	}
	if len(cfg.Overlays.URLs) > 3 {
		return fmt.Errorf("[ERR]: Too many Overlays to connect to in config (max 3)")
	}

	Cfg = &cfg
	return nil
}
