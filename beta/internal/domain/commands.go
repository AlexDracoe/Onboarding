package domain

// Command is anything app can hand to the sender: a CommandLong or a CommandInt.
// CommandName is the MAV_CMD name without the prefix, the same string the COMMAND_ACK carries back.
type Command interface {
	CommandName() string
}

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

func (c CommandLong) CommandName() string { return c.Command }

// CommandInt is a COMMAND_INT: params 5-7 become X/Y (lat/long in degrees * 1e7) and Z, with a frame.
// Frame is the MAV_FRAME name without the prefix, e.g. "GLOBAL_RELATIVE_ALT_INT".
type CommandInt struct {
	SystemID uint8
	Command  string
	Frame    string
	Param1   float32
	Param2   float32
	Param3   float32
	Param4   float32
	X        int32
	Y        int32
	Z        float32
}

func (c CommandInt) CommandName() string { return c.Command }
