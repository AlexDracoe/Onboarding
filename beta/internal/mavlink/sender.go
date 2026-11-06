package mavlink

import (
	"fmt"

	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
	"github.com/bluenviron/gomavlib/v3/pkg/message"

	"beta/internal/domain"
)

// messageWriter is the one Node method we need; *gomavlib.Node satisfies it, tests fake it
type messageWriter interface {
	WriteMessageAll(m message.Message) error
}

// Sender turns doamin commands into gomavlib messages. It implements app.CommandSender
type Sender struct {
	w messageWriter
}

func NewSender(w messageWriter) *Sender {
	return &Sender{w: w}
}

func (s *Sender) SendCommandLong(c domain.CommandLong) error {
	var cmd common.MAV_CMD
	if err := cmd.UnmarshalText([]byte("MAV_CMD_" + c.Command)); err != nil {
		return fmt.Errorf("command %q: %w", c.Command, err)
	}
	return s.w.WriteMessageAll(&common.MessageCommandLong{
		TargetSystem:    c.SystemID,
		TargetComponent: uint8(common.MAV_COMP_ID_AUTOPILOT1),
		Command:         cmd,
		Param1:          c.Param1,
		Param2:          c.Param2,
		Param3:          c.Param3,
		Param4:          c.Param4,
		Param5:          c.Param5,
		Param6:          c.Param6,
		Param7:          c.Param7,
	})
}

func (s *Sender) SendCommandInt(c domain.CommandInt) error {
	var cmd common.MAV_CMD
	if err := cmd.UnmarshalText([]byte("MAV_CMD_" + c.Command)); err != nil {
		return fmt.Errorf("command %q: %w", c.Command, err)
	}
	var frame common.MAV_FRAME
	if err := frame.UnmarshalText([]byte("MAV_FRAME_" + c.Frame)); err != nil {
		return fmt.Errorf("frame %q: %w", c.Frame, err)
	}
	return s.w.WriteMessageAll(&common.MessageCommandInt{
		TargetSystem:    c.SystemID,
		TargetComponent: uint8(common.MAV_COMP_ID_AUTOPILOT1),
		Frame:           frame,
		Command:         cmd,
		Param1:          c.Param1,
		Param2:          c.Param2,
		Param3:          c.Param3,
		Param4:          c.Param4,
		X:               c.X,
		Y:               c.Y,
		Z:               c.Z,
	})
}
