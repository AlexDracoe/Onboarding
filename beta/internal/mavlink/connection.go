package mavlink

import (
	"fmt"
	"strings"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
)

// gcsSystemID is our own MAVLink system ID, the same default pymavlink uses.
const gcsSystemID = 255

// ParseEndpoint turns a pymavlink-style connection string into a gomavlib endpoint,
// so MAVLINK_CONNECTION keeps the same format as the Python service.
// Supported: udpin:HOST:PORT, udpout:HOST:PORT, tcp:HOST:PORT, tcpin:HOST:PORT.
func ParseEndpoint(conn string) (gomavlib.EndpointConf, error) {
	scheme, addr, ok := strings.Cut(conn, ":")
	if !ok || addr == "" {
		return nil, fmt.Errorf("connection string %q: want scheme:host:port", conn)
	}

	switch scheme {
	case "udpin":
		return gomavlib.EndpointUDPServer{Address: addr}, nil
	case "udpout", "udp":
		return gomavlib.EndpointUDPClient{Address: addr}, nil
	case "tcp":
		return gomavlib.EndpointTCPClient{Address: addr}, nil
	case "tcpin":
		return gomavlib.EndpointTCPServer{Address: addr}, nil
	}
	return nil, fmt.Errorf("connection string %q: unknown scheme %q", conn, scheme)
}

// Connect opens a gomavlib Node on the given connection string with the ArduPilot dialect.
// The caller must Close it. node.Events() is closed by Close, which ends a range loop over it.
func Connect(conn string) (*gomavlib.Node, error) {
	endpoint, err := ParseEndpoint(conn)
	if err != nil {
		return nil, err
	}

	node := &gomavlib.Node{
		Endpoints:   []gomavlib.EndpointConf{endpoint},
		Dialect:     ardupilotmega.Dialect,
		OutVersion:  gomavlib.V2,
		OutSystemID: gcsSystemID,
	}
	if err := node.Initialize(); err != nil {
		return nil, fmt.Errorf("open MAVLink %q: %w", conn, err)
	}
	return node, nil
}
