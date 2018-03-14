package main

import (
	"log"
)

func main() {
	cfg := ServerConfig{}
	cfg.Host = "0.0.0.0"
	cfg.Port = 4444
	cfg.WSPort = 4445
	s, err := NewServer(&cfg)

	if err != nil {
		log.Fatalln(err)
	}

	err = s.Start()
	if err != nil {
		log.Fatalln(err)
	}
}
