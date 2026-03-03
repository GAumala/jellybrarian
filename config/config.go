package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	MediaDir        string `toml:"media_dir"`
	JellyfinMusicDir string `toml:"jellyfin_music_dir"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.MediaDir == "" {
		return nil, fmt.Errorf("media_dir must be set in config")
	}
	if cfg.JellyfinMusicDir == "" {
		return nil, fmt.Errorf("jellyfin_music_dir must be set in config")
	}
	return &cfg, nil
}
