package mavlink

import (
	"fmt"
	"strings"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"

	"beta/internal/domain"
)

// hdgUnknown is the GLOBAL_POSITION_INT hdg value for "heading not available".
const hdgUnknown = 65535

// mapFrame turns a received frame into a domain update, or nil for messages we don't map.
func mapFrame(frm *gomavlib.EventFrame) any {
	switch m := frm.Message().(type) {
	case *ardupilotmega.MessageGlobalPositionInt:
		var heading *float64
		if m.Hdg != hdgUnknown {
			heading = new(float64(m.Hdg) / 100)
		}
		return domain.GlobalPositionIntUpdate{
			SystemID: frm.SystemID(),
			Position: domain.Position{
				Latitude:         float64(m.Lat) / 1e7,
				Longitude:        float64(m.Lon) / 1e7,
				Altitude:         float64(m.Alt) / 1000,
				RelativeAltitude: float64(m.RelativeAlt) / 1000,
			},
			Velocity: domain.Velocity{
				VelocityX: float64(m.Vx) / 100,
				VelocityY: float64(m.Vy) / 100,
				VelocityZ: float64(m.Vz) / 100,
			},
			Heading: heading,
		}
	case *ardupilotmega.MessageCommandAck:
		return domain.CommandAckUpdate{
			SystemID: frm.SystemID(),
			Command:  strings.TrimPrefix(m.Command.String(), "MAV_CMD_"),
			Result:   strings.TrimPrefix(m.Result.String(), "MAV_RESULT_"),
		}
	case *ardupilotmega.MessageHeartbeat:
		return domain.HeartbeatUpdate{
			SystemID:     frm.SystemID(),
			ComponentID:  frm.ComponentID(),
			VehicleType:  strings.TrimPrefix(m.Type.String(), "MAV_TYPE_"),
			Autopilot:    strings.TrimPrefix(m.Autopilot.String(), "MAV_AUTOPILOT_"),
			IsArmed:      m.BaseMode&ardupilotmega.MAV_MODE_FLAG_SAFETY_ARMED != 0,
			FlightMode:   flightMode(m),
			SystemStatus: strings.TrimPrefix(m.SystemStatus.String(), "MAV_STATE_"),
		}
	}
	return nil
}

// flightMode names the custom mode with the ArduPilot table for the vehicle's MAV_TYPE,
// like pymavlink's mode_string_v10. Unknown types or mode numbers give "Mode(N)".
func flightMode(m *ardupilotmega.MessageHeartbeat) string {
	if m.BaseMode&ardupilotmega.MAV_MODE_FLAG_CUSTOM_MODE_ENABLED == 0 {
		return fmt.Sprintf("Mode(0x%08x)", uint8(m.BaseMode))
	}

	var name, prefix string
	switch m.Type {
	case ardupilotmega.MAV_TYPE_HELICOPTER, ardupilotmega.MAV_TYPE_TRICOPTER,
		ardupilotmega.MAV_TYPE_QUADROTOR, ardupilotmega.MAV_TYPE_HEXAROTOR,
		ardupilotmega.MAV_TYPE_OCTOROTOR, ardupilotmega.MAV_TYPE_DECAROTOR,
		ardupilotmega.MAV_TYPE_DODECAROTOR, ardupilotmega.MAV_TYPE_COAXIAL:
		name, prefix = ardupilotmega.COPTER_MODE(m.CustomMode).String(), "COPTER_MODE_"
	case ardupilotmega.MAV_TYPE_FIXED_WING, ardupilotmega.MAV_TYPE_VTOL_TAILSITTER_DUOROTOR,
		ardupilotmega.MAV_TYPE_VTOL_TAILSITTER_QUADROTOR, ardupilotmega.MAV_TYPE_VTOL_TILTROTOR:
		name, prefix = ardupilotmega.PLANE_MODE(m.CustomMode).String(), "PLANE_MODE_"
	case ardupilotmega.MAV_TYPE_GROUND_ROVER, ardupilotmega.MAV_TYPE_SURFACE_BOAT:
		name, prefix = ardupilotmega.ROVER_MODE(m.CustomMode).String(), "ROVER_MODE_"
	case ardupilotmega.MAV_TYPE_SUBMARINE:
		name, prefix = ardupilotmega.SUB_MODE(m.CustomMode).String(), "SUB_MODE_"
	case ardupilotmega.MAV_TYPE_ANTENNA_TRACKER:
		name, prefix = ardupilotmega.TRACKER_MODE(m.CustomMode).String(), "TRACKER_MODE_"
	default:
		return fmt.Sprintf("Mode(%d)", m.CustomMode)
	}

	// String() gives just the digits for a number missing from the table.
	if !strings.HasPrefix(name, prefix) {
		return fmt.Sprintf("Mode(%d)", m.CustomMode)
	}
	return strings.TrimPrefix(name, prefix)
}
