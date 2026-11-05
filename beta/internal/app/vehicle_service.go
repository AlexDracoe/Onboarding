package app

import (
	"beta/internal/config"
	"beta/internal/domain"
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrUnknownCommand = errors.New("unknown command")
var ErrStateTimeout = errors.New("state timeout")

// CommandSender sends one command to a vehicle. Mavlink,sender implements it
type CommandSender interface {
	SendCommandLong(c domain.CommandLong) error
}

// CommandSpec is one row of the command table
type CommandSpec struct {
	Build       func(systemID uint8) domain.CommandLong
	Predicate   func(s domain.VehicleState) bool
	Description string
	Timeout     time.Duration
}

// commands is the command table 5b adds a predicate and state timeout, 5c adds paramsss
var commands = map[string]CommandSpec{
	"arm": {
		Build: func(id uint8) domain.CommandLong {
			return domain.CommandLong{SystemID: id, Command: "COMPONENT_ARM_DISARM", Param1: 1}
		},
		Predicate: func(s domain.VehicleState) bool {
			return s.IsArmed != nil && *s.IsArmed
		},
		Description: "armed",
	},
	"disarm": {
		Build: func(id uint8) domain.CommandLong {
			return domain.CommandLong{SystemID: id, Command: "COMPONENT_ARM_DISARM", Param1: 0}
		},
		Predicate: func(s domain.VehicleState) bool {
			return s.IsArmed != nil && !*s.IsArmed
		},
		Description: "disarmed",
	},
}

type VehicleService struct {
	sm     *StateManager
	acks   *AckTracker
	sender CommandSender
	cfg    config.Config
}

func NewVehicleService(sm *StateManager, acks *AckTracker, sender CommandSender, cfg config.Config) *VehicleService {
	return &VehicleService{sm: sm, acks: acks, sender: sender, cfg: cfg}
}

// Execute runs one command through the ACK stage nil means the vehicle accepted it
func (vs *VehicleService) Execute(ctx context.Context, systemID uint8, name string) error {
	spec, ok := commands[name]
	if !ok {
		return fmt.Errorf("%q: %w", name, ErrUnknownCommand)
	}
	msg := spec.Build(systemID)

	p, err := vs.acks.Register(systemID, msg.Command)
	if err != nil {
		return err
	}
	if err := vs.sender.SendCommandLong(msg); err != nil {
		vs.acks.discard(p)
		return fmt.Errorf("send %s to system %d: %w", name, systemID, err)
	}

	_, err = vs.acks.WaitForAck(p, vs.cfg.AckTimeout)
	if err != nil {
		return err
	}

	timeout := spec.Timeout
	if timeout == 0 {
		timeout = vs.cfg.DefaultStateTimeout
	}
	stateCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := vs.waitForState(stateCtx, systemID, spec.Predicate); err != nil {
		return fmt.Errorf("%s on system %d after %v: %w", spec.Description, systemID, timeout, err)
	}
	return nil

}

func (vs *VehicleService) waitForState(ctx context.Context, systemID uint8, predicate func(s domain.VehicleState) bool) error {
	ticker := time.NewTicker(vs.cfg.StatePollInterval)
	defer ticker.Stop()

	for {
		if s, ok := vs.sm.Get(systemID); ok && predicate(s) {
			return nil
		}
		select {
		case <-ticker.C:
			if s, ok := vs.sm.Get(systemID); ok && predicate(s) {
				return nil
			}

		case <-ctx.Done():
			if s, ok := vs.sm.Get(systemID); ok && predicate(s) {
				return nil
			}
			return ErrStateTimeout
		}

	}

}

// StatusFor maps an Execute error to the status string stored for the command
func StatusFor(err error) string {
	var rejected *CommandRejectedError
	switch {
	case err == nil:
		return "ACHIEVED"
	case errors.Is(err, ErrAlreadyPending):
		return "ALREADY_PENDING"
	case errors.Is(err, ErrAckTimeout):
		return "ACK_TIMEOUT"
	case errors.As(err, &rejected):
		return "COMMAND_REJECTED"
	case errors.Is(err, ErrStateTimeout):
		return "STATE_TIMEOUT"
	}

	return "FAILED"
}
