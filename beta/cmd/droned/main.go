package main

import (
	"encoding/json"
	"log"
	"time"

	"beta/internal/app"
	"beta/internal/config"
	"beta/internal/mavlink"
)

func main() {
	cfg := config.Load()

	node, err := mavlink.Connect(cfg.ConnectionString)
	if err != nil {
		log.Fatal(err)
	}
	defer node.Close()
	log.Printf("mavlink: listening on %s", cfg.ConnectionString)

	sm := app.NewStateManager()
	at := app.NewAckTracker()
	r := mavlink.NewReceiver(node.Events(), sm, at)
	go r.Run()

	// Temporary until the HTTP API (Phase 7): print every vehicle once a second.
	for {
		time.Sleep(time.Second)
		for _, v := range sm.List() {
			b, _ := json.Marshal(v)
			log.Print(string(b))
		}
	}
}
