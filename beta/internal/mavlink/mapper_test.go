package mavlink

import (
	"math"
	"testing"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
	"github.com/bluenviron/gomavlib/v3/pkg/frame"
	"github.com/bluenviron/gomavlib/v3/pkg/message"

	"beta/internal/domain"
)

// frameFrom wraps a message the way gomavlib delivers it on node.Events().
func frameFrom(systemID, componentID byte, msg message.Message) *gomavlib.EventFrame {
	return &gomavlib.EventFrame{Frame: &frame.V2Frame{SystemID: systemID, ComponentID: componentID, Message: msg}}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestMapGlobalPositionInt(t *testing.T) {
	// Values as SITL sends them near CMAC (Canberra).
	msg := &ardupilotmega.MessageGlobalPositionInt{
		Lat: -353632621, Lon: 1491652374, Alt: 584090, RelativeAlt: 10020,
		Vx: 150, Vy: -25, Vz: -3, Hdg: 35999,
	}
	got, ok := mapFrame(frameFrom(2, 1, msg)).(domain.GlobalPositionIntUpdate)
	if !ok {
		t.Fatalf("got %T, want domain.GlobalPositionIntUpdate", mapFrame(frameFrom(2, 1, msg)))
	}
	if got.SystemID != 2 {
		t.Errorf("SystemID = %d, want 2", got.SystemID)
	}
	p, v := got.Position, got.Velocity
	checks := []struct {
		name      string
		got, want float64
	}{
		{"latitude", p.Latitude, -35.3632621},
		{"longitude", p.Longitude, 149.1652374},
		{"altitude", p.Altitude, 584.09},
		{"relative_altitude", p.RelativeAltitude, 10.02},
		{"velocity_x", v.VelocityX, 1.5},
		{"velocity_y", v.VelocityY, -0.25},
		{"velocity_z", v.VelocityZ, -0.03},
	}
	for _, c := range checks {
		if !near(c.got, c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if got.Heading == nil || !near(*got.Heading, 359.99) {
		t.Errorf("heading = %v, want 359.99", got.Heading)
	}
}

func TestMapGlobalPositionIntUnknownHeading(t *testing.T) {
	msg := &ardupilotmega.MessageGlobalPositionInt{Hdg: 65535}
	got := mapFrame(frameFrom(1, 1, msg)).(domain.GlobalPositionIntUpdate)
	if got.Heading != nil {
		t.Errorf("heading = %v, want nil for hdg 65535", *got.Heading)
	}
}

func TestMapCommandAck(t *testing.T) {
	tests := []struct {
		cmd        common.MAV_CMD // the ACK's field is common.MAV_CMD, not ardupilotmega's extended enum
		result     ardupilotmega.MAV_RESULT
		wantCmd    string
		wantResult string
	}{
		{common.MAV_CMD_COMPONENT_ARM_DISARM, ardupilotmega.MAV_RESULT_ACCEPTED, "COMPONENT_ARM_DISARM", "ACCEPTED"},
		{common.MAV_CMD_DO_SET_MODE, ardupilotmega.MAV_RESULT_TEMPORARILY_REJECTED, "DO_SET_MODE", "TEMPORARILY_REJECTED"},
		{common.MAV_CMD_NAV_TAKEOFF, ardupilotmega.MAV_RESULT_IN_PROGRESS, "NAV_TAKEOFF", "IN_PROGRESS"},
		{common.MAV_CMD_DO_REPOSITION, ardupilotmega.MAV_RESULT_DENIED, "DO_REPOSITION", "DENIED"},
	}
	for _, tt := range tests {
		msg := &ardupilotmega.MessageCommandAck{Command: tt.cmd, Result: tt.result}
		got, ok := mapFrame(frameFrom(3, 1, msg)).(domain.CommandAckUpdate)
		if !ok {
			t.Fatalf("%s: got %T, want domain.CommandAckUpdate", tt.wantCmd, mapFrame(frameFrom(3, 1, msg)))
		}
		want := domain.CommandAckUpdate{SystemID: 3, Command: tt.wantCmd, Result: tt.wantResult}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	}
}

func TestMapHeartbeat(t *testing.T) {
	armed := ardupilotmega.MAV_MODE_FLAG_SAFETY_ARMED | ardupilotmega.MAV_MODE_FLAG_CUSTOM_MODE_ENABLED
	disarmed := ardupilotmega.MAV_MODE_FLAG_CUSTOM_MODE_ENABLED

	tests := []struct {
		name string
		msg  *ardupilotmega.MessageHeartbeat
		want domain.HeartbeatUpdate
	}{
		{
			name: "armed copter in GUIDED",
			msg: &ardupilotmega.MessageHeartbeat{
				Type: ardupilotmega.MAV_TYPE_QUADROTOR, Autopilot: ardupilotmega.MAV_AUTOPILOT_ARDUPILOTMEGA,
				BaseMode: armed, CustomMode: 4, SystemStatus: ardupilotmega.MAV_STATE_ACTIVE,
			},
			want: domain.HeartbeatUpdate{
				SystemID: 1, ComponentID: 1, VehicleType: "QUADROTOR", Autopilot: "ARDUPILOTMEGA",
				IsArmed: true, FlightMode: "GUIDED", SystemStatus: "ACTIVE",
			},
		},
		{
			name: "disarmed copter in STABILIZE (mode 0)",
			msg: &ardupilotmega.MessageHeartbeat{
				Type: ardupilotmega.MAV_TYPE_HEXAROTOR, Autopilot: ardupilotmega.MAV_AUTOPILOT_ARDUPILOTMEGA,
				BaseMode: disarmed, CustomMode: 0, SystemStatus: ardupilotmega.MAV_STATE_STANDBY,
			},
			want: domain.HeartbeatUpdate{
				SystemID: 1, ComponentID: 1, VehicleType: "HEXAROTOR", Autopilot: "ARDUPILOTMEGA",
				IsArmed: false, FlightMode: "STABILIZE", SystemStatus: "STANDBY",
			},
		},
		{
			// Mode 15 is GUIDED for a plane but AUTOTUNE for a copter: the table must follow MAV_TYPE.
			name: "plane uses the plane table",
			msg: &ardupilotmega.MessageHeartbeat{
				Type: ardupilotmega.MAV_TYPE_FIXED_WING, Autopilot: ardupilotmega.MAV_AUTOPILOT_ARDUPILOTMEGA,
				BaseMode: disarmed, CustomMode: 15, SystemStatus: ardupilotmega.MAV_STATE_STANDBY,
			},
			want: domain.HeartbeatUpdate{
				SystemID: 1, ComponentID: 1, VehicleType: "FIXED_WING", Autopilot: "ARDUPILOTMEGA",
				IsArmed: false, FlightMode: "GUIDED", SystemStatus: "STANDBY",
			},
		},
		{
			name: "unknown mode number",
			msg: &ardupilotmega.MessageHeartbeat{
				Type: ardupilotmega.MAV_TYPE_QUADROTOR, BaseMode: disarmed, CustomMode: 200,
			},
			want: domain.HeartbeatUpdate{
				SystemID: 1, ComponentID: 1, VehicleType: "QUADROTOR", Autopilot: "GENERIC",
				FlightMode: "Mode(200)", SystemStatus: "UNINIT",
			},
		},
		{
			name: "vehicle type without an ArduPilot table (GCS)",
			msg: &ardupilotmega.MessageHeartbeat{
				Type: ardupilotmega.MAV_TYPE_GCS, BaseMode: disarmed, CustomMode: 4,
			},
			want: domain.HeartbeatUpdate{
				SystemID: 1, ComponentID: 1, VehicleType: "GCS", Autopilot: "GENERIC",
				FlightMode: "Mode(4)", SystemStatus: "UNINIT",
			},
		},
		{
			name: "custom mode flag not set",
			msg: &ardupilotmega.MessageHeartbeat{
				Type: ardupilotmega.MAV_TYPE_QUADROTOR, BaseMode: ardupilotmega.MAV_MODE_FLAG_SAFETY_ARMED,
			},
			want: domain.HeartbeatUpdate{
				SystemID: 1, ComponentID: 1, VehicleType: "QUADROTOR", Autopilot: "GENERIC",
				IsArmed: true, FlightMode: "Mode(0x00000080)", SystemStatus: "UNINIT",
			},
		},
	}
	for _, tt := range tests {
		got, ok := mapFrame(frameFrom(1, 1, tt.msg)).(domain.HeartbeatUpdate)
		if !ok {
			t.Fatalf("%s: got %T, want domain.HeartbeatUpdate", tt.name, mapFrame(frameFrom(1, 1, tt.msg)))
		}
		if got != tt.want {
			t.Errorf("%s:\n got  %+v\n want %+v", tt.name, got, tt.want)
		}
	}
}

func TestMapUnmappedReturnsNil(t *testing.T) {
	if got := mapFrame(frameFrom(1, 1, &ardupilotmega.MessageAttitude{})); got != nil {
		t.Errorf("ATTITUDE mapped to %T, want nil", got)
	}
}
