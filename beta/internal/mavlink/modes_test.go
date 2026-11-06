package mavlink

import "testing"

func TestModeNumber(t *testing.T) {
	for _, c := range []struct {
		vehicleType, mode string
		want              uint32
		ok                bool
	}{
		{"QUADROTOR", "STABILIZE", 0, true},
		{"QUADROTOR", "GUIDED", 4, true},
		{"QUADROTOR", "LOITER", 5, true},
		{"QUADROTOR", "RTL", 6, true},
		{"QUADROTOR", "LAND", 9, true},
		{"HEXAROTOR", "GUIDED", 4, true},
		{"FIXED_WING", "GUIDED", 15, true},
		{"FIXED_WING", "LOITER", 12, true},
		{"GROUND_ROVER", "GUIDED", 15, true},
		{"QUADROTOR", "WARP", 0, false},
		{"QUADROTOR", "", 0, false},
		{"QUADROTOR", "guided", 0, false}, // names are exact, like the mapper's output
		{"SPACESHIP", "GUIDED", 0, false},
		{"GCS", "GUIDED", 0, false}, // a real MAV_TYPE with no mode table
	} {
		got, ok := Modes{}.ModeNumber(c.vehicleType, c.mode)
		if got != c.want || ok != c.ok {
			t.Errorf("ModeNumber(%s, %s) = (%d, %v), want (%d, %v)", c.vehicleType, c.mode, got, ok, c.want, c.ok)
		}
	}
}
