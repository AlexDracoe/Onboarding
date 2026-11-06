package app

import (
	"beta/internal/config"
	"beta/internal/domain"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"
)

var ErrUnknownCommand = errors.New("unknown command")
var ErrStateTimeout = errors.New("state timeout")

// CommandSender sends one command to a vehicle. Mavlink,sender implements it
type CommandSender interface {
	SendCommandLong(c domain.CommandLong) error
	SendCommandInt(c domain.CommandInt) error
}

// ModeResolver turns a mode name into the vehicle's custom_mode number. mavlink.Modes implements it.
type ModeResolver interface {
	ModeNumber(vehicleType, mode string) (uint32, bool)
}
type CommandLog interface {
	Create(systemID uint8, name string, params any) (id string, err error)
	UpdateStatus(id, status string, cmdErr error) error
}

// request is what Prepare gets: who, the raw body, and what a check may need at submit time.
type request struct {
	SystemID uint8
	Body     []byte
	State    *domain.VehicleState // nil = no telemetry from this vehicle yet
	Modes    ModeResolver
}

type Bound struct {
	Params    any
	Msg       domain.Command // a CommandLong or a CommandInt
	Predicate func(s domain.VehicleState) bool
}

// CommandSpec is one row of the command table
type CommandSpec struct {
	Prepare     func(r request) (Bound, error)
	Description string
	Timeout     time.Duration
}

// commands is the command table 5b adds a predicate and state timeout, 5c adds paramsss
var commands = map[string]CommandSpec{
	"arm": {
		Prepare: func(r request) (Bound, error) {
			var p NoParams
			if err := decodeStrict(r.Body, &p); err != nil {
				return Bound{}, err // Bound{} = zero value, like returning "" with an error
			}
			return Bound{
				Params: p,
				Msg:    domain.CommandLong{SystemID: r.SystemID, Command: "COMPONENT_ARM_DISARM", Param1: 1},
				Predicate: func(s domain.VehicleState) bool {
					return s.IsArmed != nil && *s.IsArmed
				},
			}, nil
		},
		Description: "armed",
	},
	"disarm": {
		Prepare: func(r request) (Bound, error) {
			var p NoParams
			if err := decodeStrict(r.Body, &p); err != nil {
				return Bound{}, err
			}
			return Bound{
				Params: p,
				Msg:    domain.CommandLong{SystemID: r.SystemID, Command: "COMPONENT_ARM_DISARM", Param1: 0},
				Predicate: func(s domain.VehicleState) bool {
					return s.IsArmed != nil && !*s.IsArmed
				},
			}, nil
		},
		Description: "disarmed",
	},
	"takeoff": {
		Prepare: func(r request) (Bound, error) {
			var p TakeOffParams
			if err := decodeStrict(r.Body, &p); err != nil {
				return Bound{}, err
			}
			if err := p.validate(); err != nil {
				return Bound{}, err
			}
			alt := *p.Altitude
			return Bound{
				Params: p,
				Msg:    domain.CommandLong{SystemID: r.SystemID, Command: "NAV_TAKEOFF", Param4: float32(math.NaN()), Param7: float32(alt)},
				Predicate: func(s domain.VehicleState) bool {
					return s.Position != nil && s.Position.RelativeAltitude >= 0.9*alt
				},
			}, nil
		},
		Description: "took off",
		Timeout:     30 * time.Second,
	},
}

type VehicleService struct {
	sm         *StateManager
	acks       *AckTracker
	sender     CommandSender
	modes      ModeResolver
	cfg        config.Config
	commandLog CommandLog
}

func NewVehicleService(sm *StateManager, acks *AckTracker, sender CommandSender, modes ModeResolver, commandLog CommandLog, cfg config.Config) *VehicleService {
	return &VehicleService{sm: sm, acks: acks, sender: sender, modes: modes, commandLog: commandLog, cfg: cfg}
}

func (vs *VehicleService) Submit(systemID uint8, name string, body []byte) (string, error) {
	spec, ok := commands[name]
	if !ok {
		return "", fmt.Errorf("%q: %w", name, ErrUnknownCommand)
	}
	r := request{SystemID: systemID, Body: body, Modes: vs.modes}
	if s, ok := vs.sm.Get(systemID); ok {
		r.State = &s
	}
	b, err := spec.Prepare(r)
	if err != nil {
		return "", err
	}
	id, err := vs.commandLog.Create(systemID, name, b.Params)
	if err != nil {
		return "", fmt.Errorf("record %s for system %d: %w", name, systemID, err)
	}
	go vs.execute(id, systemID, name, spec, b)

	return id, nil
}

func (vs *VehicleService) execute(id string, systemID uint8, name string, spec CommandSpec, b Bound) {
	status := "FAILED"
	var cmdErr error

	defer func() {
		if r := recover(); r != nil {
			status, cmdErr = "FAILED", fmt.Errorf("panic: %v", r)
			log.Printf("command %s (%s on system %d) panicked: %v", id, name, systemID, r)
		}
		if err := vs.commandLog.UpdateStatus(id, status, cmdErr); err != nil {
			log.Printf("command %s: could not record %s (%v): %v", id, status, cmdErr, err)
		}
	}()

	// TODO(phase 8): use the service's root ctx instead of Background.
	cmdErr = vs.Execute(context.Background(), systemID, spec, b)
	status = StatusFor(cmdErr)
	if cmdErr != nil {
		log.Printf("command %s: %s: %v", id, status, cmdErr)
	}
}

// Execute runs one bound command: register, send, wait for the ACK, then for the predicate.
func (vs *VehicleService) Execute(ctx context.Context, systemID uint8, spec CommandSpec, b Bound) error {
	p, err := vs.acks.Register(systemID, b.Msg.CommandName())
	if err != nil {
		return err
	}
	if err := vs.send(b.Msg); err != nil {
		vs.acks.discard(p)
		return fmt.Errorf("send %s to system %d: %w", b.Msg.CommandName(), systemID, err)
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

	if err := vs.waitForState(stateCtx, systemID, b.Predicate); err != nil {
		return fmt.Errorf("%s on system %d after %v: %w", spec.Description, systemID, timeout, err)
	}
	return nil
}

// send picks the sender method by the message's concrete type.
func (vs *VehicleService) send(m domain.Command) error {
	switch c := m.(type) {
	case domain.CommandLong:
		return vs.sender.SendCommandLong(c)
	case domain.CommandInt:
		return vs.sender.SendCommandInt(c)
	}
	return fmt.Errorf("no sender for %T", m)
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
