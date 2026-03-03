package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"

	"github.com/BurntSushi/toml"
)

type Config struct {
	MediaDir string `toml:"media_dir"`
}

func loadConfig(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.MediaDir == "" {
		return nil, fmt.Errorf("media_dir must be set in config")
	}
	return &cfg, nil
}

// listMedia returns directory entries sorted by modification time (oldest first),
// equivalent to ls -tr.
func listMedia(mediaDir string) ([]string, error) {
	entries, err := os.ReadDir(mediaDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read media dir: %w", err)
	}

	type entry struct {
		name    string
		modTime int64
	}

	items := make([]entry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, entry{name: e.Name(), modTime: info.ModTime().UnixNano()})
	}

	// Sort by modification time ascending (oldest first) — like ls -tr
	sort.Slice(items, func(i, j int) bool {
		return items[i].modTime < items[j].modTime
	})

	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.name
	}
	return names, nil
}

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	addr := flag.String("addr", ":8090", "listen address")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("media dir: %s", cfg.MediaDir)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /media/list", func(w http.ResponseWriter, r *http.Request) {
		names, err := listMedia(cfg.MediaDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(names)
	})

	log.Printf("listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
