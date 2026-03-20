package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// rawConfig is the top-level config.toml shape. Jellyfin path keys are optional;
// each may be a string or []string (single string becomes a one-element slice).
type rawConfig struct {
	Media          string `toml:"media"`
	JellyfinMusic  any    `toml:"jellyfin_music"`
	JellyfinMovies any    `toml:"jellyfin_movies"`
	JellyfinTV     any    `toml:"jellyfin_tv"`
}

// Config is the normalized shape after Load. Jellyfin slices are never nil
// (possibly empty). After a successful Load, media exists as a directory and
// every jellyfin path entry is non-empty and exists as a directory.
type Config struct {
	Media          string
	JellyfinMusic  []string
	JellyfinMovies []string
	JellyfinTV     []string
}

func Load(path string) (*Config, error) {
	var raw rawConfig
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	result, err := refineConfig(raw)
	if err != nil {
		return nil, err
	}
	if err := result.validate(); err != nil {
		return nil, err
	}
	return &result, nil
}

func refineConfig(raw rawConfig) (Config, error) {
	music, err := optionalStringListFromAny("jellyfin_music", raw.JellyfinMusic)
	if err != nil {
		return Config{}, err
	}
	movies, err := optionalStringListFromAny("jellyfin_movies", raw.JellyfinMovies)
	if err != nil {
		return Config{}, err
	}
	tv, err := optionalStringListFromAny("jellyfin_tv", raw.JellyfinTV)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Media:          raw.Media,
		JellyfinMusic:  music,
		JellyfinMovies: movies,
		JellyfinTV:     tv,
	}, nil
}

func optionalStringListFromAny(key string, v any) ([]string, error) {
	if v == nil {
		return []string{}, nil
	}
	switch x := v.(type) {
	case string:
		return []string{x}, nil
	case []any:
		out := make([]string, 0, len(x))
		for i, el := range x {
			s, ok := el.(string)
			if !ok {
				return nil, fmt.Errorf("%s[%d]: expected string, got %T", key, i, el)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s: expected string or array of strings, got %T", key, v)
	}
}

func (cfg Config) validate() error {
	if err := validateDir(cfg.Media, "media"); err != nil {
		return err
	}
	if err := validateAllDirs(cfg.JellyfinMusic, "jellyfin_music"); err != nil {
		return err
	}
	if err := validateAllDirs(cfg.JellyfinMovies, "jellyfin_movies"); err != nil {
		return err
	}
	if err := validateAllDirs(cfg.JellyfinTV, "jellyfin_tv"); err != nil {
		return err
	}
	return nil
}

func validateDir(path, label string) error {
	if path == "" {
		return fmt.Errorf("%s: path must not be empty", label)
	}

	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if !st.IsDir() {
		return fmt.Errorf("%s: %q is not a directory", label, path)
	}
	return nil
}

func validateAllDirs(paths []string, label string) error {
	for i, p := range paths {
		indexedLabel := fmt.Sprintf("%s[%d]", label, i)
		if err := validateDir(p, indexedLabel); err != nil {
			return err
		}

	}
	return nil
}