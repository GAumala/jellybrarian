package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Directories struct {
	Media          string `toml:"media"`
	JellyfinMusic  string `toml:"jellyfin_music"`
	JellyfinMovies string `toml:"jellyfin_movies"`
	JellyfinTV     string `toml:"jellyfin_tv"`
}

type Config struct {
	Directories Directories `toml:"directories"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if err := cfg.Directories.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (d Directories) validate() error {
	required := map[string]string{
		"directories.media":           d.Media,
		"directories.jellyfin_music":  d.JellyfinMusic,
		"directories.jellyfin_movies": d.JellyfinMovies,
		"directories.jellyfin_tv":     d.JellyfinTV,
	}
	for key, val := range required {
		if val == "" {
			return fmt.Errorf("%s must be set in config", key)
		}
	}
	return nil
}
