package main

import (
	"log"
	"net/http"

	"beta/internal/api"
	"beta/internal/app"
	"beta/internal/config"
	"beta/internal/mavlink"
	"beta/internal/store"
)

func main() {
	cfg := config.Load()

	node, err := mavlink.Connect(cfg.ConnectionString)
	if err != nil {
		log.Fatal(err)
	}
	defer node.Close()
	log.Printf("mavlink: listening on %s", cfg.ConnectionString)

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	log.Printf("store: %s", cfg.DBPath)

	sm := app.NewStateManager()
	at := app.NewAckTracker()
	r := mavlink.NewReceiver(node.Events(), sm, at)
	go r.Run()

	vs := app.NewVehicleService(sm, at, mavlink.NewSender(node), db, cfg)

	// TODO(phase 8): graceful shutdown (signal.NotifyContext + srv.Shutdown).
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: api.NewHandler(sm, vs, db)}
	log.Printf("http: listening on %s", cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Print(err) // not log.Fatal: that would skip the deferred Close calls
	}
}
