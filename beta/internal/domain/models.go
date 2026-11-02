package domain

import "time"

type Position struct {
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	Altitude         float64 `json:"altitude"`
	RelativeAltitude float64 `json:"relative_altitude"`
}

type Velocity struct {
	VelocityX float64 `json:"velocity_x"`
	VelocityY float64 `json:"velocity_y"`
	VelocityZ float64 `json:"velocity_z"`
}

type VehicleState struct {
	SystemID       uint8     `json:"system_id"`
	ComponentID    *uint8    `json:"component_id"`
	VehicleType    *string   `json:"vehicle_type"`
	Autopilot      *string   `json:"autopilot"`
	IsArmed        *bool     `json:"is_armed"`
	FlightMode     *string   `json:"flight_mode"`
	SystemStatus   *string   `json:"system_status"`
	HeartbeatCount int       `json:"heartbeat_count"`
	Position       *Position `json:"position"`
	Velocity       *Velocity `json:"velocity"`
	Heading        *float64  `json:"heading"`

	LastHeartbeatUpdateTime      time.Time `json:"-"`
	LastGlobalPositionUpdateTime time.Time `json:"-"`
}
