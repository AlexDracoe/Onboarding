package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
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
			if c.Command == "NAV_TAKEOFF" { // climb straight to the commanded altitude
				f.sm.Apply(domain.GlobalPositionIntUpdate{SystemID: c.SystemID,
					Position: domain.Position{RelativeAltitude: float64(c.Param7)}})
				return
			}
			f.sm.Apply(domain.HeartbeatUpdate{SystemID: c.SystemID, IsArmed: c.Param1 == 1})
		}()
	}
	return nil
}

func (f *fakeVehicle) lastSent() domain.CommandLong {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sent[len(f.sent)-1]
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
	params   string // cmd.params as JSON
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

func (l *fakeLog) Create(systemID uint8, name string, params any) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.createErr != nil {
		return "", l.createErr
	}
	b, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	l.n++
	id := fmt.Sprintf("cmd-%d", l.n)
	l.rows[id] = row{systemID: systemID, name: name, params: string(b), status: "PENDING"}
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

// prepare runs the table's Prepare for name with body and fails the test on an error.
func prepare(t *testing.T, name string, systemID uint8, body string) (CommandSpec, Bound) {
	t.Helper()
	spec, ok := commands[name]
	if !ok {
		t.Fatalf("no %q in the command table", name)
	}
	b, err := spec.Prepare(request{SystemID: systemID, Body: []byte(body)})
	if err != nil {
		t.Fatalf("Prepare %s %s: %v", name, body, err)
	}
	return spec, b
}

// execute prepares name with an empty body and runs Execute with a hang guard.
func execute(t *testing.T, vs *VehicleService, systemID uint8, name string) error {
	t.Helper()
	return executeCtx(t, context.Background(), vs, systemID, name, "")
}

func executeCtx(t *testing.T, ctx context.Context, vs *VehicleService, systemID uint8, name, body string) error {
	t.Helper()
	spec, b := prepare(t, name, systemID, body)
	var err error
	returnsWithin(t, "Execute "+name, func() { err = vs.Execute(ctx, systemID, spec, b) })
	return err
}

func TestTableBuildsArmDisarm(t *testing.T) {
	for _, c := range []struct {
		name   string
		param1 float32
	}{{"arm", 1}, {"disarm", 0}} {
		for _, body := range []string{"", "{}", " {} "} {
			_, b := prepare(t, c.name, 7, body)
			want := domain.CommandLong{SystemID: 7, Command: "COMPONENT_ARM_DISARM", Param1: c.param1}
			if b.Msg != want {
				t.Errorf("%s %q: got %+v, want %+v", c.name, body, b.Msg, want)
			}
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
		_, b := prepare(t, c.name, 1, "")
		got := b.Predicate(domain.VehicleState{IsArmed: c.isArmed})
		if got != c.want {
			t.Errorf("%s predicate with IsArmed=%v: got %v, want %v", c.name, c.isArmed, got, c.want)
		}
	}
}

func TestTakeoffBuild(t *testing.T) {
	_, b := prepare(t, "takeoff", 3, `{"altitude": 10}`)
	m := b.Msg
	if m.SystemID != 3 || m.Command != "NAV_TAKEOFF" || m.Param7 != 10 {
		t.Errorf("msg = %+v, want NAV_TAKEOFF for system 3 with Param7 10", m)
	}
	if !math.IsNaN(float64(m.Param4)) {
		t.Errorf("Param4 = %v, want NaN (yaw: don't care)", m.Param4)
	}
	if m.Param1 != 0 || m.Param2 != 0 || m.Param3 != 0 || m.Param5 != 0 || m.Param6 != 0 {
		t.Errorf("msg = %+v, want params 1-3, 5, 6 zero", m)
	}
	for _, body := range []string{`{"altitude": 2}`, `{"altitude": 100}`} {
		prepare(t, "takeoff", 1, body) // the range ends are allowed
	}
}

func TestTakeoffPredicate(t *testing.T) {
	_, b := prepare(t, "takeoff", 1, `{"altitude": 10}`)
	at := func(alt float64) domain.VehicleState {
		return domain.VehicleState{Position: &domain.Position{RelativeAltitude: alt}}
	}
	for _, c := range []struct {
		desc string
		s    domain.VehicleState
		want bool
	}{
		{"no position yet", domain.VehicleState{}, false}, // must not panic
		{"on the ground", at(0), false},
		{"just below 90%", at(8.9), false},
		{"at 90%", at(9), true},
		{"above", at(12), true},
	} {
		if got := b.Predicate(c.s); got != c.want {
			t.Errorf("%s: got %v, want %v", c.desc, got, c.want)
		}
	}
}

func TestPrepareRejectsBadParams(t *testing.T) {
	for _, c := range []struct {
		name, body string
		want       string // substring of the 422 detail
	}{
		{"arm", `{"altitude": 10}`, `unknown field "altitude"`},
		{"arm", `[1, 2]`, "body:"},
		{"arm", `null`, "body: must be a JSON object"},
		{"arm", `nope`, "body:"},
		{"disarm", `{"x": 1}`, `unknown field "x"`},
		{"takeoff", ``, "altitude: required"},
		{"takeoff", `{}`, "altitude: required"},
		{"takeoff", `{"altitude": null}`, "altitude: required"},
		{"takeoff", `{"altitude": 1.9}`, "altitude: must be between 2 and 100"},
		{"takeoff", `{"altitude": 100.5}`, "altitude: must be between 2 and 100"},
		{"takeoff", `{"altitude": "10"}`, "body:"},
		{"takeoff", `{"altitde": 10}`, `unknown field "altitde"`},
	} {
		_, err := commands[c.name].Prepare(request{SystemID: 1, Body: []byte(c.body)})
		var pe *ParamsError
		if !errors.As(err, &pe) {
			t.Errorf("%s %s: got %v, want a *ParamsError", c.name, c.body, err)
			continue
		}
		if !strings.Contains(pe.Error(), c.want) {
			t.Errorf("%s %s: detail %q, want it to mention %q", c.name, c.body, pe.Error(), c.want)
		}
	}
}

func TestParamsErrorJoinsWithSemicolon(t *testing.T) {
	err := &ParamsError{Problems: []string{"lat: too big", "long: too small"}}
	if got := err.Error(); got != "lat: too big;long: too small" {
		t.Errorf("got %q", got)
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
	err := executeCtx(t, ctx, vs, 1, "arm", "")
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
	id, err := vs.Submit(1, "arm", nil)
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
			id, err := vs.Submit(1, "arm", nil)
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
	id, err := vs.Submit(1, "fly", nil)
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

	id, err := vs.Submit(1, "arm", nil)
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

	id, err := vs.Submit(1, "arm", nil)
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

	if _, err := vs.Submit(1, "arm", nil); err != nil {
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

func TestSubmitInvalidParamsLeavesNoRow(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED", reachAt: 0}
	vs, fl := newSubmitService(f, testConfig())
	id, err := vs.Submit(1, "takeoff", []byte(`{"altitude": 500}`))
	var pe *ParamsError
	if !errors.As(err, &pe) || id != "" {
		t.Errorf("got (%q, %v), want (\"\", *ParamsError)", id, err)
	}
	time.Sleep(20 * time.Millisecond)
	if fl.count() != 0 || f.sentCount() != 0 {
		t.Errorf("rows=%d sent=%d, want 0 and 0", fl.count(), f.sentCount())
	}
}

func TestSubmitStoresDecodedParams(t *testing.T) {
	for _, c := range []struct {
		name, body, want string
	}{
		{"arm", "", "{}"},
		{"arm", "{}", "{}"},
		{"takeoff", `{ "altitude" : 10.0 }`, `{"altitude":10}`},
	} {
		f := &fakeVehicle{ackResult: "ACCEPTED", reachAt: 0}
		vs, fl := newSubmitService(f, testConfig())
		id, err := vs.Submit(1, c.name, []byte(c.body))
		if err != nil {
			t.Fatalf("Submit %s %s: %v", c.name, c.body, err)
		}
		if r, _ := fl.get(id); r.params != c.want {
			t.Errorf("%s %s: stored params %s, want %s", c.name, c.body, r.params, c.want)
		}
		finalRow(t, fl, id)
	}
}

func TestSubmitTakeoffAchieved(t *testing.T) {
	f := &fakeVehicle{ackResult: "IN_PROGRESS", reachAt: 30 * time.Millisecond}
	vs, fl := newSubmitService(f, testConfig())
	id, err := vs.Submit(1, "takeoff", []byte(`{"altitude": 10}`))
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if r := finalRow(t, fl, id); r.status != "ACHIEVED" {
		t.Errorf("final row = %+v, want ACHIEVED", r)
	}
	if m := f.lastSent(); m.Command != "NAV_TAKEOFF" || m.Param7 != 10 {
		t.Errorf("sent %+v, want NAV_TAKEOFF to 10 m", m)
	}
}

// takeoff has its own 30 s state timeout; it must not fall back to the (short) default.
func TestTakeoffUsesItsOwnTimeout(t *testing.T) {
	if got := commands["takeoff"].Timeout; got != 30*time.Second {
		t.Errorf("takeoff timeout = %v, want 30s", got)
	}
}

// The vehicle is known but stays disarmed: only the bound predicate can tell it is not done.
func TestKnownVehicleWrongStateTimesOut(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED", reachAt: -1}
	vs := newTestService(f, testConfig())
	f.sm.Apply(domain.HeartbeatUpdate{SystemID: 1, IsArmed: false})
	if err := execute(t, vs, 1, "arm"); !errors.Is(err, ErrStateTimeout) {
		t.Errorf("got %v, want ErrStateTimeout", err)
	}
}
