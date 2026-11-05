package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
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
	vs, _ := newSubmitService(f, cfg)
	return vs
}

func newSubmitService(f *fakeVehicle, cfg config.Config) (*VehicleService, *fakeLog) {
	f.sm, f.at = NewStateManager(), NewAckTracker()
	fl := &fakeLog{rows: map[string]row{}}
	return NewVehicleService(f.sm, f.at, f, fl, cfg), fl
}

// row is one command record in the fake log.
type row struct {
	systemID uint8
	name     string
	status   string
	err      string // "" = NULL
}

// fakeLog is an in-memory CommandLog. createErr/updateErr make the "DB" fail.
type fakeLog struct {
	mu        sync.Mutex
	rows      map[string]row
	n         int
	createErr error
	updateErr error
}

func (l *fakeLog) Create(systemID uint8, name string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.createErr != nil {
		return "", l.createErr
	}
	l.n++
	id := fmt.Sprintf("cmd-%d", l.n)
	l.rows[id] = row{systemID: systemID, name: name, status: "PENDING"}
	return id, nil
}

func (l *fakeLog) UpdateStatus(id, status string, cmdErr error) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.updateErr != nil {
		return l.updateErr
	}
	r := l.rows[id]
	r.status, r.err = status, ""
	if cmdErr != nil {
		r.err = cmdErr.Error()
	}
	l.rows[id] = r
	return nil
}

func (l *fakeLog) get(id string) (row, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r, ok := l.rows[id]
	return r, ok
}

func (l *fakeLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.rows)
}

// finalRow waits (max 1s) until the row leaves PENDING and returns it.
func finalRow(t *testing.T, l *fakeLog, id string) row {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if r, _ := l.get(id); r.status != "PENDING" {
			return r
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("command %s still PENDING after 1s", id)
	return row{}
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

func TestSubmitReturnsBeforeCommandFinishes(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED", reachAt: 150 * time.Millisecond}
	vs, fl := newSubmitService(f, testConfig())

	start := time.Now()
	id, err := vs.Submit(1, "arm")
	if took := time.Since(start); took > 50*time.Millisecond {
		t.Errorf("Submit took %v: it must return before the command finishes (missing go?)", took)
	}
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if r, _ := fl.get(id); r.status != "PENDING" || r.systemID != 1 || r.name != "arm" {
		t.Errorf("row right after Submit = %+v, want PENDING arm for system 1", r)
	}
	if r := finalRow(t, fl, id); r.status != "ACHIEVED" || r.err != "" {
		t.Errorf("final row = %+v, want ACHIEVED with no error", r)
	}
}

func TestSubmitFinalStatus(t *testing.T) {
	for _, c := range []struct {
		desc    string
		f       *fakeVehicle
		want    string
		wantErr string // substring of the stored error, "" = must be NULL
	}{
		{"achieved", &fakeVehicle{ackResult: "ACCEPTED", reachAt: 0}, "ACHIEVED", ""},
		{"rejected", &fakeVehicle{ackResult: "DENIED", reachAt: -1}, "COMMAND_REJECTED", "DENIED"},
		{"no ack", &fakeVehicle{reachAt: -1}, "ACK_TIMEOUT", "no ACK"},
		{"never armed", &fakeVehicle{ackResult: "ACCEPTED", reachAt: -1}, "STATE_TIMEOUT", "armed on system 1"},
		{"send fails", &fakeVehicle{err: errors.New("link down")}, "FAILED", "link down"},
	} {
		t.Run(c.desc, func(t *testing.T) {
			vs, fl := newSubmitService(c.f, testConfig())
			id, err := vs.Submit(1, "arm")
			if err != nil {
				t.Fatalf("Submit: %v", err)
			}
			r := finalRow(t, fl, id)
			if r.status != c.want {
				t.Errorf("status = %s (err %q), want %s", r.status, r.err, c.want)
			}
			if c.wantErr == "" && r.err != "" {
				t.Errorf("error column = %q, want NULL", r.err)
			}
			if c.wantErr != "" && !strings.Contains(r.err, c.wantErr) {
				t.Errorf("error column = %q, want it to mention %q", r.err, c.wantErr)
			}
		})
	}
}

func TestSubmitUnknownCommandLeavesNoRow(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED"}
	vs, fl := newSubmitService(f, testConfig())
	id, err := vs.Submit(1, "fly")
	if !errors.Is(err, ErrUnknownCommand) || id != "" {
		t.Errorf("got (%q, %v), want (\"\", ErrUnknownCommand)", id, err)
	}
	time.Sleep(20 * time.Millisecond)
	if fl.count() != 0 || f.sentCount() != 0 {
		t.Errorf("rows=%d sent=%d, want 0 and 0", fl.count(), f.sentCount())
	}
}

func TestSubmitCreateFailsStartsNothing(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED", reachAt: 0}
	vs, fl := newSubmitService(f, testConfig())
	dbErr := errors.New("disk full")
	fl.createErr = dbErr

	id, err := vs.Submit(1, "arm")
	if !errors.Is(err, dbErr) || id != "" {
		t.Errorf("got (%q, %v), want (\"\", wrapped disk full)", id, err)
	}
	time.Sleep(20 * time.Millisecond)
	if n := f.sentCount(); n != 0 {
		t.Errorf("sent %d commands although the row was never created", n)
	}
}

type panickingSender struct{}

func (panickingSender) SendCommandLong(domain.CommandLong) error {
	var m map[string]int
	m["boom"] = 1 // nil map write: panics
	return nil
}

func TestPanicEndsAsFailedNotPending(t *testing.T) {
	fl := &fakeLog{rows: map[string]row{}}
	vs := NewVehicleService(NewStateManager(), NewAckTracker(), panickingSender{}, fl, testConfig())
	logs := captureLog(t)

	id, err := vs.Submit(1, "arm")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	r := finalRow(t, fl, id)
	if r.status != "FAILED" || !strings.Contains(r.err, "panic") {
		t.Errorf("row = %+v, want FAILED with a panic error", r)
	}
	if !strings.Contains(logs.String(), "panicked") {
		t.Errorf("panic not logged; log was %q", logs.String())
	}
}

// When the final write itself fails, both the DB error and the command's error must reach the log.
func TestUpdateStatusFailureIsLogged(t *testing.T) {
	f := &fakeVehicle{err: errors.New("link down")}
	vs, fl := newSubmitService(f, testConfig())
	fl.updateErr = errors.New("db locked")
	logs := captureLog(t)

	if _, err := vs.Submit(1, "arm"); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(logs.String(), "db locked") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	out := logs.String()
	if !strings.Contains(out, "db locked") || !strings.Contains(out, "link down") {
		t.Errorf("log should mention the DB error and the command error; got %q", out)
	}
}

// syncBuffer is a bytes.Buffer safe for the log package and the test to share.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// captureLog sends the standard logger to a buffer for the rest of the test.
func captureLog(t *testing.T) *syncBuffer {
	buf := &syncBuffer{}
	old := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(old) })
	return buf
}
