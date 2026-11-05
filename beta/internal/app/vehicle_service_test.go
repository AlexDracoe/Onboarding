package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"beta/internal/config"
	"beta/internal/domain"
)

// fakeVehicle plays the drone: it records every command it is sent, optionally ACKs it
// (through the real AckTracker), and optionally reports the commanded arm state after a delay.
type fakeVehicle struct {
	sm  *StateManager
	at  *AckTracker
	err error // returned by SendCommandLong instead of sending

	ackResult string        // "" = never ACK
	reachAt   time.Duration // < 0 = never reach the commanded state

	mu   sync.Mutex
	sent []domain.CommandLong
}

func (f *fakeVehicle) SendCommandLong(c domain.CommandLong) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	f.sent = append(f.sent, c)
	f.mu.Unlock()

	if f.ackResult != "" {
		go f.at.HandleAck(ack(c.SystemID, c.Command, f.ackResult))
	}
	if f.reachAt >= 0 {
		go func() {
			time.Sleep(f.reachAt)
			f.sm.Apply(domain.HeartbeatUpdate{SystemID: c.SystemID, IsArmed: c.Param1 == 1})
		}()
	}
	return nil
}

func (f *fakeVehicle) sentCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func testConfig() config.Config {
	cfg := config.Load()
	cfg.AckTimeout = shortTimeout
	cfg.DefaultStateTimeout = 200 * time.Millisecond
	cfg.StatePollInterval = 10 * time.Millisecond
	return cfg
}

func newTestService(f *fakeVehicle, cfg config.Config) *VehicleService {
	f.sm, f.at = NewStateManager(), NewAckTracker()
	return NewVehicleService(f.sm, f.at, f, cfg)
}

// execute runs Execute with a hang guard and returns its error.
func execute(t *testing.T, vs *VehicleService, systemID uint8, name string) error {
	t.Helper()
	var err error
	returnsWithin(t, "Execute "+name, func() { err = vs.Execute(context.Background(), systemID, name) })
	return err
}

func TestTableBuildsArmDisarm(t *testing.T) {
	for _, c := range []struct {
		name   string
		param1 float32
	}{{"arm", 1}, {"disarm", 0}} {
		msg := commands[c.name].Build(7)
		want := domain.CommandLong{SystemID: 7, Command: "COMPONENT_ARM_DISARM", Param1: c.param1}
		if msg != want {
			t.Errorf("%s: got %+v, want %+v", c.name, msg, want)
		}
	}
}

func TestArmDisarmPredicates(t *testing.T) {
	yes, no := true, false
	for _, c := range []struct {
		name    string
		isArmed *bool
		want    bool
	}{
		{"arm", nil, false}, // no heartbeat yet: must not panic, must not count as armed
		{"arm", &yes, true},
		{"arm", &no, false},
		{"disarm", nil, false}, // nil is "unknown", not "disarmed"
		{"disarm", &yes, false},
		{"disarm", &no, true},
	} {
		got := commands[c.name].Predicate(domain.VehicleState{IsArmed: c.isArmed})
		if got != c.want {
			t.Errorf("%s predicate with IsArmed=%v: got %v, want %v", c.name, c.isArmed, got, c.want)
		}
	}
}

func TestExecuteOutcomes(t *testing.T) {
	for _, c := range []struct {
		desc string
		f    *fakeVehicle
		cmd  string
		want string
	}{
		{"accepted and armed", &fakeVehicle{ackResult: "ACCEPTED", reachAt: 0}, "arm", "ACHIEVED"},
		{"in progress then armed", &fakeVehicle{ackResult: "IN_PROGRESS", reachAt: 30 * time.Millisecond}, "arm", "ACHIEVED"},
		{"disarm", &fakeVehicle{ackResult: "ACCEPTED", reachAt: 0}, "disarm", "ACHIEVED"},
		{"rejected", &fakeVehicle{ackResult: "DENIED", reachAt: -1}, "arm", "COMMAND_REJECTED"},
		{"no ack", &fakeVehicle{reachAt: -1}, "arm", "ACK_TIMEOUT"},
		{"accepted, never armed", &fakeVehicle{ackResult: "ACCEPTED", reachAt: -1}, "arm", "STATE_TIMEOUT"},
		{"send fails", &fakeVehicle{err: errors.New("link down")}, "arm", "FAILED"},
		{"unknown command", &fakeVehicle{ackResult: "ACCEPTED"}, "fly", "FAILED"},
	} {
		t.Run(c.desc, func(t *testing.T) {
			vs := newTestService(c.f, testConfig())
			err := execute(t, vs, 1, c.cmd)
			if got := StatusFor(err); got != c.want {
				t.Errorf("got %s (err: %v), want %s", got, err, c.want)
			}
			if n := len(c.f.at.pending); n != 0 {
				t.Errorf("%d entries left in the AckTracker", n)
			}
		})
	}
}

func TestUnknownCommandSendsNothing(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED"}
	err := execute(t, newTestService(f, testConfig()), 1, "fly")
	if !errors.Is(err, ErrUnknownCommand) {
		t.Errorf("got %v, want ErrUnknownCommand", err)
	}
	if n := f.sentCount(); n != 0 {
		t.Errorf("sent %d messages for an unknown command", n)
	}
}

func TestFailedSendFreesKeyForRetry(t *testing.T) {
	f := &fakeVehicle{err: errors.New("link down")}
	vs := newTestService(f, testConfig())
	if err := execute(t, vs, 1, "arm"); StatusFor(err) != "FAILED" {
		t.Fatalf("first try: got %v, want a send error", err)
	}
	f.err, f.ackResult, f.reachAt = nil, "ACCEPTED", 0
	if err := execute(t, vs, 1, "arm"); err != nil {
		t.Errorf("retry after a failed send: got %v, want nil (key still registered?)", err)
	}
}

func TestStateTimeoutUsesDefault(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED", reachAt: -1}
	cfg := testConfig()
	vs := newTestService(f, cfg)

	start := time.Now()
	err := execute(t, vs, 1, "arm")
	took := time.Since(start)

	if !errors.Is(err, ErrStateTimeout) {
		t.Fatalf("got %v, want ErrStateTimeout", err)
	}
	if took < cfg.DefaultStateTimeout || took > cfg.DefaultStateTimeout+time.Second {
		t.Errorf("took %v, want about %v (the default state timeout)", took, cfg.DefaultStateTimeout)
	}
}

// The state arrives after the last tick but before the deadline: only the final check sees it.
func TestFinalCheckAfterDeadline(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED", reachAt: 50 * time.Millisecond}
	cfg := testConfig()
	cfg.StatePollInterval = 5 * time.Second // never ticks before the deadline
	cfg.DefaultStateTimeout = 150 * time.Millisecond
	vs := newTestService(f, cfg)

	if err := execute(t, vs, 1, "arm"); err != nil {
		t.Errorf("got %v, want nil: state was reached before the deadline", err)
	}
}

// Execute obeys the caller's ctx (Phase 8 shutdown), not just its own state timeout.
func TestCallerCancelStopsWait(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED", reachAt: -1}
	cfg := testConfig()
	cfg.DefaultStateTimeout = time.Hour
	vs := newTestService(f, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var err error
	returnsWithin(t, "Execute with a cancelled parent", func() { err = vs.Execute(ctx, 1, "arm") })
	if !errors.Is(err, ErrStateTimeout) {
		t.Errorf("got %v, want ErrStateTimeout", err)
	}
}

func TestStatusFor(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{nil, "ACHIEVED"},
		{ErrAlreadyPending, "ALREADY_PENDING"},
		{ErrAckTimeout, "ACK_TIMEOUT"},
		{&CommandRejectedError{Result: "DENIED"}, "COMMAND_REJECTED"},
		{ErrStateTimeout, "STATE_TIMEOUT"},
		{errors.New("anything else"), "FAILED"},
	} {
		if got := StatusFor(c.err); got != c.want {
			t.Errorf("StatusFor(%v) = %s, want %s", c.err, got, c.want)
		}
	}
}
