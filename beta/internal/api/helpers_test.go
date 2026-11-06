package api

import (
	"time"

	"beta/internal/config"
	"beta/internal/domain"
)

type silentSender struct{}

func (silentSender) SendCommandLong(domain.CommandLong) error { return nil }
func (silentSender) SendCommandInt(domain.CommandInt) error   { return nil }

type noModes struct{}

func (noModes) ModeNumber(string, string) (uint32, bool) { return 0, false }

func testConfig() config.Config {
	cfg := config.Load()
	cfg.AckTimeout = 50 * time.Millisecond
	cfg.StatePollInterval = 10 * time.Millisecond
	return cfg
}

func sleep() { time.Sleep(10 * time.Millisecond) }
