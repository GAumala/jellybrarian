package main

import (
	"flag"
	"log"
	"net/http"

	"media-manager/config"
	"media-manager/server"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	addr := flag.String("addr", ":8090", "listen address")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	d := cfg.Directories
	log.Printf("media dir:           %s", d.Media)
	log.Printf("jellyfin music dir:  %s", d.JellyfinMusic)
	log.Printf("jellyfin movies dir: %s", d.JellyfinMovies)
	log.Printf("jellyfin tv dir:     %s", d.JellyfinTV)

	handler := server.New(cfg)

	log.Printf("listening on %s", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal(err)
	}
}
