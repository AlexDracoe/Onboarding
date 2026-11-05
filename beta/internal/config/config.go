package config

import (
	"os"
	"time"
)

type Config struct {
	AckTimeout          time.Duration `json:"ack_timeout"`
	ReceiveBackoff      time.Duration `json:"receive_backoff"`
	ReceiveTimeout      time.Duration `json:"receive_timeout"`
	DefaultStateTimeout time.Duration `json:"-"`
	StatePollInterval   time.Duration `json:"state_poll_interval"`
	StartupTimeout      time.Duration `json:"startup_timeout"`
	ConnectionString    string        `json:"connection_string"`
	DBPath              string        `json:"db_path"`
	HTTPAddr            string        `json:"http_addr"`
}

func Load() Config {
	return Config{
		AckTimeout:          10 * time.Second,
		ReceiveBackoff:      500 * time.Millisecond,
		ReceiveTimeout:      1 * time.Second,
		DefaultStateTimeout: 20 * time.Second,
		StatePollInterval:   100 * time.Millisecond,
		StartupTimeout:      10 * time.Second,
		ConnectionString:    getEnv("MAVLINK_CONNECTION", "udpin:127.0.0.1:14540"),
		DBPath:              getEnv("DB_PATH", "drone.db"),
		HTTPAddr:            getEnv("HTTP_ADDR", ":8000"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
