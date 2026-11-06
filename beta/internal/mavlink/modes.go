package mavlink

import (
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
)

// Modes turns a flight mode name into ArduPilot's custom_mode number. It implements app.ModeResolver.
// The vehicle type picks the table, the same split flightMode() in mapper.go uses for the other direction.
type Modes struct{}

// ModeNumber: vehicleType is VehicleState.VehicleType ("QUADROTOR"), mode is a name like "GUIDED".
// ok is false for an unknown vehicle type or a mode that type doesn't have.
func (Modes) ModeNumber(vehicleType, mode string) (uint32, bool) {
	var t ardupilotmega.MAV_TYPE
	if err := t.UnmarshalText([]byte("MAV_TYPE_" + vehicleType)); err != nil {
		return 0, false
	}
	switch t {
	case ardupilotmega.MAV_TYPE_HELICOPTER, ardupilotmega.MAV_TYPE_TRICOPTER,
		ardupilotmega.MAV_TYPE_QUADROTOR, ardupilotmega.MAV_TYPE_HEXAROTOR,
		ardupilotmega.MAV_TYPE_OCTOROTOR, ardupilotmega.MAV_TYPE_DECAROTOR,
		ardupilotmega.MAV_TYPE_DODECAROTOR, ardupilotmega.MAV_TYPE_COAXIAL:
		var m ardupilotmega.COPTER_MODE
		err := m.UnmarshalText([]byte("COPTER_MODE_" + mode))
		return uint32(m), err == nil
	case ardupilotmega.MAV_TYPE_FIXED_WING, ardupilotmega.MAV_TYPE_VTOL_TAILSITTER_DUOROTOR,
		ardupilotmega.MAV_TYPE_VTOL_TAILSITTER_QUADROTOR, ardupilotmega.MAV_TYPE_VTOL_TILTROTOR:
		var m ardupilotmega.PLANE_MODE
		err := m.UnmarshalText([]byte("PLANE_MODE_" + mode))
		return uint32(m), err == nil
	case ardupilotmega.MAV_TYPE_GROUND_ROVER, ardupilotmega.MAV_TYPE_SURFACE_BOAT:
		var m ardupilotmega.ROVER_MODE
		err := m.UnmarshalText([]byte("ROVER_MODE_" + mode))
		return uint32(m), err == nil
	case ardupilotmega.MAV_TYPE_SUBMARINE:
		var m ardupilotmega.SUB_MODE
		err := m.UnmarshalText([]byte("SUB_MODE_" + mode))
		return uint32(m), err == nil
	case ardupilotmega.MAV_TYPE_ANTENNA_TRACKER:
		var m ardupilotmega.TRACKER_MODE
		err := m.UnmarshalText([]byte("TRACKER_MODE_" + mode))
		return uint32(m), err == nil
	}
	return 0, false
}
