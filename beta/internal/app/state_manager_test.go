package app

import (
	"encoding/json"
	"sync"
	"testing"

	"beta/internal/domain"
)

// TestStateManagerRace hammers Apply from 3 writers while 3 readers call Get/List and
// read through their copies. Run with -race: any shared write the lock doesn't cover fails it.
func TestStateManagerRace(t *testing.T) {
	sm := NewStateManager()
	var wg sync.WaitGroup
	for id := uint8(1); id <= 3; id++ {
		wg.Go(func() {
			for i := range 2000 {
				sm.Apply(domain.HeartbeatUpdate{SystemID: id, IsArmed: i%2 == 0, FlightMode: "GUIDED"})
				sm.Apply(domain.GlobalPositionIntUpdate{
					SystemID: id,
					Position: domain.Position{Latitude: float64(i)},
					Heading:  new(float64(i)),
				})
			}
		})
	}
	for range 3 {
		wg.Go(func() {
			for range 2000 {
				if v, ok := sm.Get(2); ok && v.Position != nil && v.IsArmed != nil {
					_ = v.Position.Latitude + *v.Heading
					_ = *v.IsArmed
				}
				for _, v := range sm.List() {
					if _, err := json.Marshal(v); err != nil {
						t.Error(err)
					}
				}
			}
		})
	}
	wg.Wait()

	if _, ok := sm.Get(99); ok {
		t.Fatal("Get(99): want not found")
	}
	l := sm.List()
	if len(l) != 3 || l[0].SystemID != 1 || l[1].SystemID != 2 || l[2].SystemID != 3 {
		t.Fatalf("List: want ids 1,2,3 in order, got %+v", l)
	}
	if l[0].HeartbeatCount != 2000 {
		t.Fatalf("HeartbeatCount = %d, want 2000", l[0].HeartbeatCount)
	}
}

// TestPositionBeforeHeartbeat: telemetry first means heartbeat fields stay null.
func TestPositionBeforeHeartbeat(t *testing.T) {
	sm := NewStateManager()
	sm.Apply(domain.GlobalPositionIntUpdate{SystemID: 1, Position: domain.Position{Latitude: 47.4}})
	sm.Apply("junk") // logged, ignored
	v, ok := sm.Get(1)
	if !ok {
		t.Fatal("Get(1): want found")
	}
	b, _ := json.Marshal(v)
	want := `{"system_id":1,"component_id":null,"vehicle_type":null,"autopilot":null,"is_armed":null,"flight_mode":null,"system_status":null,"heartbeat_count":0,"position":{"latitude":47.4,"longitude":0,"altitude":0,"relative_altitude":0},"velocity":{"velocity_x":0,"velocity_y":0,"velocity_z":0},"heading":null}`
	if string(b) != want {
		t.Fatalf("got  %s\nwant %s", b, want)
	}
}
