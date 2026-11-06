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
// (through the real AckTracker), and optionally reports the commanded state after a delay:
// armed/disarmed, the new mode, the takeoff altitude, or the goto position.
type fakeVehicle struct {
	sm  *StateManager
	at  *AckTracker
	err error // returned by the Send methods instead of sending

	ackResult string        // "" = never ACK
	reachAt   time.Duration // < 0 = never reach the commanded state

	mu   sync.Mutex
	sent []domain.Command
}

func (f *fakeVehicle) SendCommandLong(c domain.CommandLong) error {
	return f.receive(c, func() {
		switch c.Command {
		case "NAV_TAKEOFF": // climb straight to the commanded altitude
			f.sm.Apply(domain.GlobalPositionIntUpdate{SystemID: c.SystemID,
				Position: domain.Position{RelativeAltitude: float64(c.Param7)}})
		case "DO_SET_MODE":
			f.sm.Apply(domain.HeartbeatUpdate{SystemID: c.SystemID, VehicleType: "QUADROTOR",
				FlightMode: fakeModeNames[uint32(c.Param2)]})
		default:
			f.sm.Apply(domain.HeartbeatUpdate{SystemID: c.SystemID, IsArmed: c.Param1 == 1})
		}
	})
}

func (f *fakeVehicle) SendCommandInt(c domain.CommandInt) error {
	return f.receive(c, func() { // DO_REPOSITION: teleport to the target
		f.sm.Apply(domain.GlobalPositionIntUpdate{SystemID: c.SystemID, Position: domain.Position{
			Latitude: float64(c.X) / 1e7, Longitude: float64(c.Y) / 1e7, RelativeAltitude: float64(c.Z)}})
	})
}

// receive records m, ACKs it if configured, and runs reach after reachAt.
func (f *fakeVehicle) receive(m domain.Command, reach func()) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	f.sent = append(f.sent, m)
	f.mu.Unlock()

	sys := systemOf(m)
	if f.ackResult != "" {
		go f.at.HandleAck(ack(sys, m.CommandName(), f.ackResult))
	}
	if f.reachAt >= 0 {
		go func() {
			time.Sleep(f.reachAt)
			reach()
		}()
	}
	return nil
}

func systemOf(m domain.Command) uint8 {
	switch c := m.(type) {
	case domain.CommandLong:
		return c.SystemID
	case domain.CommandInt:
		return c.SystemID
	}
	return 0
}

// fakeModes is a ModeResolver with a tiny copter table.
type fakeModes struct{}

var fakeModeNumbers = map[string]uint32{"STABILIZE": 0, "GUIDED": 4, "LOITER": 5, "RTL": 6, "LAND": 9}
var fakeModeNames = map[uint32]string{0: "STABILIZE", 4: "GUIDED", 5: "LOITER", 6: "RTL", 9: "LAND"}

func (fakeModes) ModeNumber(vehicleType, mode string) (uint32, bool) {
	if vehicleType != "QUADROTOR" {
		return 0, false
	}
	n, ok := fakeModeNumbers[mode]
	return n, ok
}

func (f *fakeVehicle) lastSent() domain.Command {
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
	return NewVehicleService(f.sm, f.at, f, fakeModes{}, fl, cfg), fl
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

// prepare runs the table's Prepare for name with body (no vehicle state) and fails the test on an error.
func prepare(t *testing.T, name string, systemID uint8, body string) (CommandSpec, Bound) {
	t.Helper()
	return prepareWith(t, name, request{SystemID: systemID, Body: []byte(body), Modes: fakeModes{}})
}

// tablePrepare calls Prepare and returns its error; a command missing from the table fails the test.
func tablePrepare(t *testing.T, name string, r request) (Bound, error) {
	t.Helper()
	spec, ok := commands[name]
	if !ok {
		t.Fatalf("no %q in the command table yet", name)
	}
	return spec.Prepare(r)
}

func prepareWith(t *testing.T, name string, r request) (CommandSpec, Bound) {
	t.Helper()
	spec, ok := commands[name]
	if !ok {
		t.Fatalf("no %q in the command table yet", name)
	}
	b, err := spec.Prepare(r)
	if err != nil {
		t.Fatalf("Prepare %s %s: %v", name, r.Body, err)
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
	m, ok := b.Msg.(domain.CommandLong)
	if !ok {
		t.Fatalf("takeoff Msg is %T, want domain.CommandLong", b.Msg)
	}
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
		_, err := commands[c.name].Prepare(request{SystemID: 1, Body: []byte(c.body), Modes: fakeModes{}})
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

func (panickingSender) SendCommandInt(domain.CommandInt) error { return nil }

func (panickingSender) SendCommandLong(domain.CommandLong) error {
	var m map[string]int
	m["boom"] = 1 // nil map write: panics
	return nil
}

func TestPanicEndsAsFailedNotPending(t *testing.T) {
	fl := &fakeLog{rows: map[string]row{}}
	vs := NewVehicleService(NewStateManager(), NewAckTracker(), panickingSender{}, fakeModes{}, fl, testConfig())
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
	if m, _ := f.lastSent().(domain.CommandLong); m.Command != "NAV_TAKEOFF" || m.Param7 != 10 {
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

// ---- set_mode ----

func copter(mode string) *domain.VehicleState {
	vt := "QUADROTOR"
	return &domain.VehicleState{SystemID: 1, VehicleType: &vt, FlightMode: &mode}
}

func TestSetModeBuild(t *testing.T) {
	_, b := prepareWith(t, "set_mode", request{SystemID: 2, Body: []byte(`{"mode": "GUIDED"}`),
		State: copter("STABILIZE"), Modes: fakeModes{}})
	want := domain.CommandLong{SystemID: 2, Command: "DO_SET_MODE", Param1: 1, Param2: 4}
	if b.Msg != want {
		t.Errorf("msg = %+v, want %+v", b.Msg, want)
	}
}

func TestSetModePredicate(t *testing.T) {
	_, b := prepareWith(t, "set_mode", request{SystemID: 1, Body: []byte(`{"mode": "GUIDED"}`),
		State: copter("STABILIZE"), Modes: fakeModes{}})
	for _, c := range []struct {
		desc string
		s    domain.VehicleState
		want bool
	}{
		{"no heartbeat mode yet", domain.VehicleState{}, false}, // must not panic
		{"old mode", *copter("STABILIZE"), false},
		{"new mode", *copter("GUIDED"), true},
	} {
		if got := b.Predicate(c.s); got != c.want {
			t.Errorf("%s: got %v, want %v", c.desc, got, c.want)
		}
	}
}

func TestSetModeRejects(t *testing.T) {
	for _, c := range []struct {
		desc  string
		body  string
		state *domain.VehicleState
		want  string
	}{
		{"missing", `{}`, copter("STABILIZE"), "mode: required"},
		{"empty", `{"mode": ""}`, copter("STABILIZE"), "mode: required"},
		{"unknown key", `{"mode": "GUIDED", "x": 1}`, copter("STABILIZE"), `unknown field "x"`},
		{"not a string", `{"mode": 4}`, copter("STABILIZE"), "body:"},
		{"unknown mode", `{"mode": "WARP"}`, copter("STABILIZE"), `unknown mode "WARP"`},
		{"no telemetry yet", `{"mode": "GUIDED"}`, nil, "no heartbeat"},
		{"heartbeat without a type", `{"mode": "GUIDED"}`, &domain.VehicleState{SystemID: 1}, "no heartbeat"},
	} {
		_, err := tablePrepare(t, "set_mode", request{SystemID: 1, Body: []byte(c.body), State: c.state, Modes: fakeModes{}})
		var pe *ParamsError
		if !errors.As(err, &pe) {
			t.Errorf("%s: got %v, want a *ParamsError", c.desc, err)
			continue
		}
		if !strings.Contains(pe.Error(), c.want) {
			t.Errorf("%s: detail %q, want it to mention %q", c.desc, pe.Error(), c.want)
		}
	}
}

func TestSubmitSetModeAchieved(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED", reachAt: 20 * time.Millisecond}
	vs, fl := newSubmitService(f, testConfig())
	f.sm.Apply(domain.HeartbeatUpdate{SystemID: 1, VehicleType: "QUADROTOR", FlightMode: "STABILIZE"})

	id, err := vs.Submit(1, "set_mode", []byte(`{"mode": "GUIDED"}`))
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if r := finalRow(t, fl, id); r.status != "ACHIEVED" || r.params != `{"mode":"GUIDED"}` {
		t.Errorf("final row = %+v, want ACHIEVED with params {\"mode\":\"GUIDED\"}", r)
	}
}

// Submit must hand Prepare the vehicle's current state: without a heartbeat, set_mode is a 422, not a 202.
func TestSubmitSetModeWithoutHeartbeatIs422(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED", reachAt: 0}
	vs, fl := newSubmitService(f, testConfig())
	_, err := vs.Submit(1, "set_mode", []byte(`{"mode": "GUIDED"}`))
	var pe *ParamsError
	if !errors.As(err, &pe) || fl.count() != 0 {
		t.Errorf("got %v with %d rows, want a *ParamsError and no row", err, fl.count())
	}
}

// ---- goto ----

const homeLat, homeLong = -35.363261, 149.165230 // ArduPilot SITL default home

func hovering(lat, long, relAlt float64) *domain.VehicleState {
	return &domain.VehicleState{SystemID: 1, Position: &domain.Position{Latitude: lat, Longitude: long, RelativeAltitude: relAlt}}
}

func gotoBody(lat, long, alt float64) []byte {
	return []byte(fmt.Sprintf(`{"lat": %v, "long": %v, "altitude": %v}`, lat, long, alt))
}

func TestGotoBuild(t *testing.T) {
	lat, long := homeLat+0.001, homeLong+0.001 // ~145 m away
	_, b := prepareWith(t, "goto", request{SystemID: 4, Body: gotoBody(lat, long, 25), State: hovering(homeLat, homeLong, 10)})
	m, ok := b.Msg.(domain.CommandInt)
	if !ok {
		t.Fatalf("goto Msg is %T, want domain.CommandInt (ArduPilot needs COMMAND_INT for DO_REPOSITION)", b.Msg)
	}
	if m.SystemID != 4 || m.Command != "DO_REPOSITION" || m.Frame != "GLOBAL_RELATIVE_ALT_INT" {
		t.Errorf("msg = %+v, want DO_REPOSITION to system 4 in GLOBAL_RELATIVE_ALT_INT", m)
	}
	if m.Param1 != -1 || m.Param2 != 0 || m.Param3 != 0 || !math.IsNaN(float64(m.Param4)) {
		t.Errorf("params = %v %v %v %v, want -1 0 0 NaN", m.Param1, m.Param2, m.Param3, m.Param4)
	}
	wantX, wantY := int32(math.Round(lat*1e7)), int32(math.Round(long*1e7))
	if d := m.X - wantX; d < -1 || d > 1 {
		t.Errorf("X = %d, want %d (lat * 1e7)", m.X, wantX)
	}
	if d := m.Y - wantY; d < -1 || d > 1 {
		t.Errorf("Y = %d, want %d (long * 1e7)", m.Y, wantY)
	}
	if m.Z != 25 {
		t.Errorf("Z = %v, want 25", m.Z)
	}
}

func TestGotoPredicate(t *testing.T) {
	tLat, tLong := homeLat+0.001, homeLong
	_, b := prepareWith(t, "goto", request{SystemID: 1, Body: gotoBody(tLat, tLong, 20), State: hovering(homeLat, homeLong, 10)})
	for _, c := range []struct {
		desc string
		s    domain.VehicleState
		want bool
	}{
		{"no position", domain.VehicleState{}, false}, // must not panic
		{"at the target", *hovering(tLat, tLong, 20), true},
		{"5 m alt off", *hovering(tLat, tLong, 25), true},
		{"6 m alt off", *hovering(tLat, tLong, 14), false},
		{"~5.6 m short", *hovering(tLat-0.00005, tLong, 20), true},
		{"~11 m short", *hovering(tLat-0.0001, tLong, 20), false},
		{"still at home", *hovering(homeLat, homeLong, 20), false},
	} {
		if got := b.Predicate(c.s); got != c.want {
			t.Errorf("%s: got %v, want %v", c.desc, got, c.want)
		}
	}
}

func TestGotoRejects(t *testing.T) {
	here := hovering(homeLat, homeLong, 10)
	for _, c := range []struct {
		desc  string
		body  string
		state *domain.VehicleState
		want  string
	}{
		{"all missing", `{}`, here, "lat: required;long: required;altitude: required"},
		{"lat missing", `{"long": 149.1, "altitude": 10}`, here, "lat: required"},
		{"lat too big", `{"lat": 91, "long": 149.1, "altitude": 10}`, here, "lat: must be between -90 and 90"},
		{"long too small", `{"lat": -35, "long": -181, "altitude": 10}`, here, "long: must be between -180 and 180"},
		{"alt too low", `{"lat": -35, "long": 149, "altitude": 1}`, here, "altitude: must be between 2 and 400"},
		{"alt too high", `{"lat": -35, "long": 149, "altitude": 401}`, here, "altitude: must be between 2 and 400"},
		{"two bad fields", `{"lat": 91, "long": 181, "altitude": 10}`, here, ";long: must be between -180 and 180"}, // lat first, joined by ;
		{"unknown key", `{"lat": -35, "long": 149, "altitude": 10, "speed": 3}`, here, `unknown field "speed"`},
		{"no telemetry", string(gotoBody(homeLat, homeLong, 10)), nil, "no position fix"},
		{"no position yet", string(gotoBody(homeLat, homeLong, 10)), &domain.VehicleState{SystemID: 1}, "no position fix"},
		{"too far", string(gotoBody(homeLat+0.006, homeLong, 10)), here, "max is 500 m"}, // ~667 m
	} {
		_, err := tablePrepare(t, "goto", request{SystemID: 1, Body: []byte(c.body), State: c.state, Modes: fakeModes{}})
		var pe *ParamsError
		if !errors.As(err, &pe) {
			t.Errorf("%s: got %v, want a *ParamsError", c.desc, err)
			continue
		}
		if !strings.Contains(pe.Error(), c.want) {
			t.Errorf("%s: detail %q, want it to mention %q", c.desc, pe.Error(), c.want)
		}
	}
}

// lat 0 / long 0 are real coordinates: "missing" must come from a nil pointer, not from a zero value.
func TestGotoAcceptsZeroCoordinates(t *testing.T) {
	prepareWith(t, "goto", request{SystemID: 1, Body: gotoBody(0, 0, 10), State: hovering(0.001, 0.001, 10)})
	prepareWith(t, "goto", request{SystemID: 1, Body: gotoBody(homeLat+0.004, homeLong, 10), State: hovering(homeLat, homeLong, 10)}) // ~445 m: allowed
}

func TestSubmitGotoAchieved(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED", reachAt: 20 * time.Millisecond}
	vs, fl := newSubmitService(f, testConfig())
	f.sm.Apply(domain.GlobalPositionIntUpdate{SystemID: 1, Position: domain.Position{Latitude: homeLat, Longitude: homeLong, RelativeAltitude: 10}})

	id, err := vs.Submit(1, "goto", gotoBody(homeLat+0.001, homeLong, 20))
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if r := finalRow(t, fl, id); r.status != "ACHIEVED" || !strings.Contains(r.params, `"lat":`) {
		t.Errorf("final row = %+v, want ACHIEVED with lat/long/altitude params", r)
	}
	if _, ok := f.lastSent().(domain.CommandInt); !ok {
		t.Errorf("sent %T, want a CommandInt", f.lastSent())
	}
}

func TestGotoUsesItsOwnTimeout(t *testing.T) {
	if got := commands["goto"].Timeout; got != 200*time.Second {
		t.Errorf("goto timeout = %v, want 200s", got)
	}
}

func TestSendRoutesByType(t *testing.T) {
	f := &fakeVehicle{ackResult: "ACCEPTED", reachAt: -1}
	vs := newTestService(f, testConfig())
	for _, m := range []domain.Command{domain.CommandLong{SystemID: 1, Command: "A"}, domain.CommandInt{SystemID: 1, Command: "B"}} {
		if err := vs.send(m); err != nil {
			t.Fatal(err)
		}
		if got := f.lastSent(); got != m {
			t.Errorf("sent %+v, want %+v", got, m)
		}
	}
}
