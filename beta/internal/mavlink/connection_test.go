package mavlink

import (
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
)

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		conn string
		want gomavlib.EndpointConf
	}{
		{"udpin:0.0.0.0:14540", gomavlib.EndpointUDPServer{Address: "0.0.0.0:14540"}},
		{"udpin:127.0.0.1:14540", gomavlib.EndpointUDPServer{Address: "127.0.0.1:14540"}},
		{"udpout:127.0.0.1:14550", gomavlib.EndpointUDPClient{Address: "127.0.0.1:14550"}},
		{"udp:127.0.0.1:14550", gomavlib.EndpointUDPClient{Address: "127.0.0.1:14550"}},
		{"tcp:127.0.0.1:5760", gomavlib.EndpointTCPClient{Address: "127.0.0.1:5760"}},
		{"tcpin:0.0.0.0:5760", gomavlib.EndpointTCPServer{Address: "0.0.0.0:5760"}},
	}
	for _, tt := range tests {
		got, err := ParseEndpoint(tt.conn)
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.conn, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: got %#v, want %#v", tt.conn, got, tt.want)
		}
	}
}

func TestParseEndpointErrors(t *testing.T) {
	for _, conn := range []string{"", "udpin", "udpin:", "serial:/dev/ttyUSB0", "14540"} {
		if _, err := ParseEndpoint(conn); err == nil {
			t.Errorf("%q: expected an error", conn)
		}
	}
}

// TestConnectReceivesHeartbeat opens a node like the service does and has a second
// node play the vehicle, sending a HEARTBEAT as system 1.
func TestConnectReceivesHeartbeat(t *testing.T) {
	const addr = "127.0.0.1:24540"
	gcs, err := Connect("udpin:" + addr)
	if err != nil {
		t.Fatal(err)
	}
	defer gcs.Close()

	vehicle := &gomavlib.Node{
		Endpoints:        []gomavlib.EndpointConf{gomavlib.EndpointUDPClient{Address: addr}},
		Dialect:          ardupilotmega.Dialect,
		OutVersion:       gomavlib.V2,
		OutSystemID:      1,
		HeartbeatDisable: true,
	}
	if err := vehicle.Initialize(); err != nil {
		t.Fatal(err)
	}
	defer vehicle.Close()

	hb := &ardupilotmega.MessageHeartbeat{
		Type:       ardupilotmega.MAV_TYPE_QUADROTOR,
		Autopilot:  ardupilotmega.MAV_AUTOPILOT_ARDUPILOTMEGA,
		CustomMode: uint32(ardupilotmega.COPTER_MODE_GUIDED),
	}
	// Like a real vehicle, keep sending: a write before the UDP channel is open is dropped.
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-tick.C:
			if err := vehicle.WriteMessageAll(hb); err != nil {
				t.Fatal(err)
			}
		case evt := <-gcs.Events():
			frm, ok := evt.(*gomavlib.EventFrame)
			if !ok {
				continue
			}
			got, ok := frm.Message().(*ardupilotmega.MessageHeartbeat)
			if !ok {
				continue
			}
			if frm.SystemID() != 1 || got.CustomMode != uint32(ardupilotmega.COPTER_MODE_GUIDED) {
				t.Fatalf("got system %d mode %d", frm.SystemID(), got.CustomMode)
			}
			return
		case <-deadline:
			t.Fatal("no HEARTBEAT received within 2s")
		}
	}
}
