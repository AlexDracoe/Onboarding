package api

import (
	"time"

	"beta/internal/config"
	"beta/internal/domain"
)

type silentSender struct{}

func (silentSender) SendCommandLong(domain.CommandLong) error { return nil }

func testConfig() config.Config {
	cfg := config.Load()
	cfg.AckTimeout = 50 * time.Millisecond
	cfg.StatePollInterval = 10 * time.Millisecond
	return cfg
}

func sleep() { time.Sleep(10 * time.Millisecond) }
