package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"beta/internal/app"
	"beta/internal/domain"
	"beta/internal/store"
)

// Compile-time checks: the real types satisfy the interfaces the API declares.
var (
	_ vehicleSource    = (*app.StateManager)(nil)
	_ commandSubmitter = (*app.VehicleService)(nil)
	_ commandRecords   = (*store.SQLite)(nil)
)

type fakeSubmitter struct {
	err       error
	gotSystem uint8
	gotName   string
}

func (f *fakeSubmitter) Submit(systemID uint8, name string) (string, error) {
	f.gotSystem, f.gotName = systemID, name
	if f.err != nil {
		return "", f.err
	}
	return "cmd-1", nil
}

type fakeRecords struct {
	recs      []store.Record
	err       error
	gotSystem *uint8
	gotLimit  int
}

func (f *fakeRecords) Get(id string) (store.Record, bool, error) {
	for _, r := range f.recs {
		if r.ID == id {
			return r, true, f.err
		}
	}
	return store.Record{}, false, f.err
}

func (f *fakeRecords) List(systemID *uint8, limit int) ([]store.Record, error) {
	f.gotSystem, f.gotLimit = systemID, limit
	if f.err != nil {
		return nil, f.err
	}
	return f.recs, nil
}

func newTestServer(sub *fakeSubmitter, recs *fakeRecords) (http.Handler, *app.StateManager) {
	sm := app.NewStateManager()
	return NewHandler(sm, sub, recs), sm
}

func do(t *testing.T, h http.Handler, method, path, body string) (int, string) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, r))
	return rec.Code, strings.TrimSpace(rec.Body.String())
}

func wantError(t *testing.T, code int, body string, wantCode int, wantErr string) {
	t.Helper()
	if code != wantCode {
		t.Errorf("status = %d, want %d (body %s)", code, wantCode, body)
	}
	var e map[string]string
	if err := json.Unmarshal([]byte(body), &e); err != nil || len(e) != 2 || e["error"] != wantErr || e["detail"] == "" {
		t.Errorf("body = %s, want exactly {\"error\": %q, \"detail\": ...}", body, wantErr)
	}
}

func TestSubmitCommand(t *testing.T) {
	sub := &fakeSubmitter{}
	h, _ := newTestServer(sub, &fakeRecords{})

	code, body := do(t, h, "POST", "/vehicles/2/commands/arm", "")
	if code != http.StatusAccepted || body != `{"command_id":"cmd-1","status":"PENDING"}` {
		t.Errorf("got %d %s, want 202 with command_id + PENDING", code, body)
	}
	if sub.gotSystem != 2 || sub.gotName != "arm" {
		t.Errorf("Submit got (%d, %q), want (2, arm)", sub.gotSystem, sub.gotName)
	}

	// An empty JSON object is fine too (that is what a params-less body looks like).
	if code, body := do(t, h, "POST", "/vehicles/2/commands/arm", "{}"); code != http.StatusAccepted {
		t.Errorf("POST with {} = %d %s, want 202", code, body)
	}
}

func TestSubmitErrors(t *testing.T) {
	for _, c := range []struct {
		desc, path, body string
		subErr           error
		wantCode         int
		wantErr          string
	}{
		{"unknown command", "/vehicles/1/commands/fly", "", fmt.Errorf("%q: %w", "fly", app.ErrUnknownCommand), 404, "UNKNOWN_COMMAND"},
		{"bad system id", "/vehicles/abc/commands/arm", "", nil, 422, "INVALID_PARAMS"},
		{"system id too big", "/vehicles/256/commands/arm", "", nil, 422, "INVALID_PARAMS"},
		{"body not JSON", "/vehicles/1/commands/arm", "altitude=10", nil, 422, "INVALID_PARAMS"},
		{"body not an object", "/vehicles/1/commands/arm", "[1,2]", nil, 422, "INVALID_PARAMS"},
		{"body null", "/vehicles/1/commands/arm", "null", nil, 422, "INVALID_PARAMS"},
		{"db down", "/vehicles/1/commands/arm", "", errors.New("disk full"), 500, "INTERNAL"},
	} {
		t.Run(c.desc, func(t *testing.T) {
			h, _ := newTestServer(&fakeSubmitter{err: c.subErr}, &fakeRecords{})
			code, body := do(t, h, "POST", c.path, c.body)
			wantError(t, code, body, c.wantCode, c.wantErr)
		})
	}
}

func TestVehicles(t *testing.T) {
	h, sm := newTestServer(&fakeSubmitter{}, &fakeRecords{})

	if code, body := do(t, h, "GET", "/vehicles", ""); code != 200 || body != "[]" {
		t.Errorf("GET /vehicles with none = %d %s, want 200 []", code, body)
	}
	code, body := do(t, h, "GET", "/vehicles/1", "")
	wantError(t, code, body, 404, "NO_VEHICLE")

	sm.Apply(domain.HeartbeatUpdate{SystemID: 1, IsArmed: true, FlightMode: "GUIDED"})
	code, body = do(t, h, "GET", "/vehicles/1", "")
	var v map[string]any
	if err := json.Unmarshal([]byte(body), &v); code != 200 || err != nil {
		t.Fatalf("GET /vehicles/1 = %d %s", code, body)
	}
	if len(v) != 11 || v["is_armed"] != true || v["flight_mode"] != "GUIDED" || v["position"] != nil {
		t.Errorf("vehicle JSON = %s, want the 11 contract keys", body)
	}
	if code, body := do(t, h, "GET", "/vehicles", ""); code != 200 || !strings.HasPrefix(body, `[{"system_id":1,`) {
		t.Errorf("GET /vehicles = %d %s", code, body)
	}
}

func TestCommands(t *testing.T) {
	errText := "no ACK"
	recs := &fakeRecords{recs: []store.Record{{
		ID: "abc", SystemID: 1, Cmd: json.RawMessage(`{"name":"arm","params":{}}`),
		Status: "ACK_TIMEOUT", Error: &errText, CreatedAt: 1.5, UpdatedAt: 2.5,
	}}}
	h, _ := newTestServer(&fakeSubmitter{}, recs)

	code, body := do(t, h, "GET", "/commands/abc", "")
	want := `{"id":"abc","system_id":1,"cmd":{"name":"arm","params":{}},"status":"ACK_TIMEOUT","error":"no ACK","created_at":1.5,"updated_at":2.5}`
	if code != 200 || body != want {
		t.Errorf("GET /commands/abc = %d\n got  %s\n want %s", code, body, want)
	}
	code, body = do(t, h, "GET", "/commands/nope", "")
	wantError(t, code, body, 404, "UNKNOWN_COMMAND_ID")

	if code, _ := do(t, h, "GET", "/commands", ""); code != 200 || recs.gotLimit != 50 || recs.gotSystem != nil {
		t.Errorf("GET /commands = %d, List got (system %v, limit %d), want (nil, 50)", code, recs.gotSystem, recs.gotLimit)
	}
	if code, _ := do(t, h, "GET", "/commands?system_id=3&limit=500", ""); code != 200 || recs.gotLimit != 500 || recs.gotSystem == nil || *recs.gotSystem != 3 {
		t.Errorf("GET /commands?system_id=3&limit=500 = %d, List got limit %d", code, recs.gotLimit)
	}
	for _, q := range []string{"limit=0", "limit=501", "limit=x", "system_id=-1"} {
		code, body := do(t, h, "GET", "/commands?"+q, "")
		wantError(t, code, body, 422, "INVALID_PARAMS")
	}

	recs.err = errors.New("db gone")
	code, body = do(t, h, "GET", "/commands", "")
	wantError(t, code, body, 500, "INTERNAL")
}

// Full stack below HTTP: real StateManager, AckTracker, VehicleService and SQLite store.
func TestRealStackRoundTrip(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/drone.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sm, at := app.NewStateManager(), app.NewAckTracker()
	vs := app.NewVehicleService(sm, at, silentSender{}, db, testConfig())
	h := NewHandler(sm, vs, db)

	code, body := do(t, h, "POST", "/vehicles/1/commands/arm", "")
	var accepted map[string]string
	json.Unmarshal([]byte(body), &accepted)
	if code != 202 || accepted["command_id"] == "" {
		t.Fatalf("POST = %d %s", code, body)
	}

	// silentSender never ACKs: the row must end as ACK_TIMEOUT (ack timeout is 50 ms in testConfig).
	var rec store.Record
	for range 100 {
		_, body = do(t, h, "GET", "/commands/"+accepted["command_id"], "")
		json.Unmarshal([]byte(body), &rec)
		if rec.Status != "PENDING" {
			break
		}
		sleep()
	}
	if rec.Status != "ACK_TIMEOUT" || rec.Error == nil {
		t.Errorf("final record = %s, want ACK_TIMEOUT with an error", body)
	}
	if _, body := do(t, h, "GET", "/commands?system_id=1", ""); !strings.Contains(body, accepted["command_id"]) {
		t.Errorf("history does not list the command: %s", body)
	}
}
