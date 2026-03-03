package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/gabriel/media-manager/config"
	"github.com/gabriel/media-manager/server"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	addr := flag.String("addr", ":8090", "listen address")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("media dir: %s", cfg.MediaDir)
	log.Printf("jellyfin music dir: %s", cfg.JellyfinMusicDir)

	handler := server.New(cfg)

	log.Printf("listening on %s", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal(err)
	}
}
