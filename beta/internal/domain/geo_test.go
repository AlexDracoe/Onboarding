package domain

import (
	"math"
	"testing"
)

// Expected values come from the Python reference (alpha/domain/geo.py).
func TestDistance(t *testing.T) {
	tests := []struct {
		name                     string
		lat1, long1, lat2, long2 float64
		want                     float64
	}{
		{"same point", 47.3977419, 8.5455938, 47.3977419, 8.5455938, 0},
		{"1 deg latitude at equator", 0, 0, 1, 0, 111194.92664455874},
		{"1 deg longitude at equator", 0, 0, 0, 1, 111194.92664455874},
		{"~100 m east at 47N", 47.3977419, 8.5455938, 47.3977419, 8.5469190, 99.74568309762533},
		{"Sydney to London", -33.8688, 151.2093, 51.5074, -0.1278, 16993933.459795907},
		{"across the antimeridian", 0, 179.9, 0, -179.9, 22238.985328911924},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Distance(tc.lat1, tc.long1, tc.lat2, tc.long2)
			if math.Abs(got-tc.want) > 1e-6 {
				t.Errorf("Distance = %.6f, want %.6f", got, tc.want)
			}
		})
	}
}
