package config

import (
	"os"
	"time"
)

type Config struct {
	AckTimeout        time.Duration
	ReceiveBackoff    time.Duration
	ReceiveTimeout    time.Duration
	StatePollInterval time.Duration
	StartupTimeout    time.Duration
	ConnectionString  string
	DBPath            string
	HTTPAddr          string
}

func Load() Config {
	return Config{
		AckTimeout:        10 * time.Second,
		ReceiveBackoff:    500 * time.Millisecond,
		ReceiveTimeout:    1 * time.Second,
		StatePollInterval: 100 * time.Millisecond,
		StartupTimeout:    10 * time.Second,
		ConnectionString:  getEnv("MAVLINK_CONNECTION", "udpin:127.0.0.1:14540"),
		DBPath:            getEnv("DB_PATH", "drone.db"),
		HTTPAddr:          getEnv("HTTP_ADDR", ":8000"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
