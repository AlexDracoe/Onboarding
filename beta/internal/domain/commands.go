package domain

// CommandLong is a COMMAND_LONG with no MAVLink library types: app builds it, mavlink sends it.
// Command is the MAV_CMD name without the prefix, the same string the COMMAND_ACK carries back.
type CommandLong struct {
	SystemID uint8
	Command  string
	Param1   float32
	Param2   float32
	Param3   float32
	Param4   float32
	Param5   float32
	Param6   float32
	Param7   float32
}
