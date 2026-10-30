package domain

type HeartbeatUpdate struct {
	SystemID     uint8
	ComponentID  uint8
	VehicleType  string
	Autopilot    string
	IsArmed      bool
	FlightMode   string
	SystemStatus string
}

type GlobalPositionIntUpdate struct {
	SystemID uint8
	Position Position
	Velocity Velocity
	Heading  *float64
}

type CommandAckUpdate struct {
	SystemID uint8
	Command  string
	Result   string
}
