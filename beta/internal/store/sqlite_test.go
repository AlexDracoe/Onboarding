package store

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func openTemp(t *testing.T) *SQLite {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "drone.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateGetUpdate(t *testing.T) {
	s := openTemp(t)
	before := float64(time.Now().Unix()) - 1

	id, err := s.Create(3, "arm", nil)
	if err != nil {
		t.Fatal(err)
	}
	rec, ok, err := s.Get(id)
	if err != nil || !ok {
		t.Fatalf("Get(%s) = ok %v, err %v", id, ok, err)
	}
	if rec.ID != id || rec.SystemID != 3 || rec.Status != "PENDING" || rec.Error != nil {
		t.Errorf("new row = %+v, want PENDING for system 3 with NULL error", rec)
	}
	if string(rec.Cmd) != `{"name":"arm","params":{}}` {
		t.Errorf("cmd = %s, want the name + empty params object", rec.Cmd)
	}
	if rec.CreatedAt < before || rec.CreatedAt != rec.UpdatedAt {
		t.Errorf("times = %v / %v, want now and equal", rec.CreatedAt, rec.UpdatedAt)
	}

	time.Sleep(5 * time.Millisecond)
	if err := s.UpdateStatus(id, "ACK_TIMEOUT", errors.New("no ACK")); err != nil {
		t.Fatal(err)
	}
	rec, _, _ = s.Get(id)
	if rec.Status != "ACK_TIMEOUT" || rec.Error == nil || *rec.Error != "no ACK" || rec.UpdatedAt <= rec.CreatedAt {
		t.Errorf("after update = %+v (error %v), want ACK_TIMEOUT / 'no ACK' / later updated_at", rec, rec.Error)
	}

	if err := s.UpdateStatus(id, "ACHIEVED", nil); err != nil {
		t.Fatal(err)
	}
	if rec, _, _ = s.Get(id); rec.Error != nil {
		t.Errorf("nil cmdErr should store NULL, got %q", *rec.Error)
	}
}

func TestUnknownID(t *testing.T) {
	s := openTemp(t)
	if _, ok, err := s.Get("nope"); ok || err != nil {
		t.Errorf("Get(nope) = ok %v, err %v; want false, nil", ok, err)
	}
	if err := s.UpdateStatus("nope", "ACHIEVED", nil); !errors.Is(err, ErrUnknownID) {
		t.Errorf("UpdateStatus(nope) = %v, want ErrUnknownID", err)
	}
}

func TestListOrderFilterLimit(t *testing.T) {
	s := openTemp(t)
	if recs, err := s.List(nil, 50); err != nil || recs == nil || len(recs) != 0 {
		t.Fatalf("empty List = %v, %v; want a non-nil empty slice (JSON [])", recs, err)
	}

	var ids []string
	for i, sys := range []uint8{1, 2, 1, 1} {
		id, err := s.Create(sys, []string{"arm", "arm", "disarm", "arm"}[i], nil)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}

	all, _ := s.List(nil, 50)
	if len(all) != 4 || all[0].ID != ids[3] || all[3].ID != ids[0] {
		t.Errorf("List(all) not newest first: %v", recordIDs(all))
	}
	one := uint8(1)
	sys1, _ := s.List(&one, 50)
	if len(sys1) != 3 || sys1[0].ID != ids[3] || sys1[2].ID != ids[0] {
		t.Errorf("List(system 1) = %v, want 3 rows newest first", recordIDs(sys1))
	}
	two, _ := s.List(nil, 2)
	if len(two) != 2 || two[0].ID != ids[3] {
		t.Errorf("List(limit 2) = %v", recordIDs(two))
	}
}

func TestRecordJSON(t *testing.T) {
	s := openTemp(t)
	id, _ := s.Create(1, "disarm", nil)
	rec, _, _ := s.Get(id)
	b, _ := json.Marshal(rec)

	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"id", "system_id", "cmd", "status", "error", "created_at", "updated_at"} {
		if _, ok := got[k]; !ok {
			t.Errorf("key %q missing from %s", k, b)
		}
	}
	if len(got) != 7 || got["error"] != nil || got["cmd"].(map[string]any)["name"] != "disarm" {
		t.Errorf("record JSON = %s", b)
	}
}

func TestConcurrentWriters(t *testing.T) {
	s := openTemp(t)
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			id, err := s.Create(1, "arm", nil)
			if err != nil {
				t.Error(err)
				return
			}
			if err := s.UpdateStatus(id, "ACHIEVED", nil); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if recs, _ := s.List(nil, 500); len(recs) != 20 {
		t.Errorf("got %d rows, want 20", len(recs))
	}
}

// The schema must match the Python service's table exactly.
func TestSchemaMatchesPython(t *testing.T) {
	s := openTemp(t)
	rows, err := s.db.Query("SELECT name, type, \"notnull\", pk FROM pragma_table_info('commands') ORDER BY cid")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var name, typ string
		var notNull, pk int
		rows.Scan(&name, &typ, &notNull, &pk)
		got = append(got, name+" "+typ+map[int]string{0: "", 1: " NOT NULL"}[notNull]+map[int]string{0: "", 1: " PK"}[pk])
	}
	want := []string{"id TEXT PK", "created_at REAL NOT NULL", "updated_at REAL NOT NULL",
		"system_id INTEGER NOT NULL", "cmd TEXT NOT NULL", "status TEXT NOT NULL", "error TEXT"}
	if len(got) != len(want) {
		t.Fatalf("columns = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("column %d = %q, want %q", i, got[i], want[i])
		}
	}
	var mode string
	s.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" {
		t.Errorf("journal_mode = %s, want wal", mode)
	}
}

func recordIDs(recs []Record) []string {
	var ids []string
	for _, r := range recs {
		ids = append(ids, r.ID[:8])
	}
	return ids
}

func TestCreateStoresParams(t *testing.T) {
	s := openTemp(t)
	alt := 10.0
	id, err := s.Create(1, "takeoff", struct {
		Altitude *float64 `json:"altitude"`
	}{&alt})
	if err != nil {
		t.Fatal(err)
	}
	rec, _, _ := s.Get(id)
	if string(rec.Cmd) != `{"name":"takeoff","params":{"altitude":10}}` {
		t.Errorf("cmd = %s", rec.Cmd)
	}
	if _, err := s.Create(1, "bad", func() {}); err == nil {
		t.Error("Create with unencodable params: want an error")
	}
}
