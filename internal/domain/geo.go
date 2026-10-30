package domain

import "math"

const EarthRadiusM = 6371000.0

func Distance(latitude1, longitude1, latitude2, longitude2 float64) float64 {
	radianLatitude1 := latitude1 * (math.Pi / 180.0)
	radianLatitude2 := latitude2 * (math.Pi / 180.0)

	deltaLatitude := radianLatitude2 - radianLatitude1
	deltaLongitude := (longitude2 - longitude1) * (math.Pi / 180.0)

	x := math.Sin(deltaLatitude / 2.0)
	y := math.Cos(radianLatitude1) * math.Cos(radianLatitude2)
	z := math.Sin(deltaLongitude / 2.0)

	a := (x * x) + (y * (z * z))

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadiusM * c
}
