package main

import (
	"fmt"
	"log"

	"github.com/jclem/jclem.me/internal/www"
)

func main() {
	server, err := www.New()
	if err != nil {
		log.Fatal(fmt.Errorf("error creating server: %w", err))
	}

	log.Fatal(server.Start())
}
