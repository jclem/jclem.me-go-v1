package main

import (
	"fmt"
	"log"

	"github.com/jclem/jclem.me/internal/www"
	"github.com/jclem/jclem.me/internal/www/config"
)

func main() {
	if _, err := config.LoadConfig(); err != nil {
		log.Fatal(fmt.Errorf("error loading config: %w", err))
	}

	server, err := www.New()
	if err != nil {
		log.Fatal(fmt.Errorf("error creating server: %w", err))
	}

	log.Fatal(server.Start())
}
