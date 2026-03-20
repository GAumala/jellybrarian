package main

import (
	"flag"
	"log"
	"net/http"

	"jellybrarian/config"
	"jellybrarian/server"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	addr := flag.String("addr", ":8090", "listen address")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("media dir: %s", cfg.Media)
	log.Printf("jellyfin music:  %v", cfg.JellyfinMusic)
	log.Printf("jellyfin movies: %v", cfg.JellyfinMovies)
	log.Printf("jellyfin tv:     %v", cfg.JellyfinTV)

	handler := server.New(cfg)

	log.Printf("listening on %s", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal(err)
	}
}
