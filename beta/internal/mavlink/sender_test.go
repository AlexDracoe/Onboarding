package mavlink

import (
	"errors"
	"math"
	"testing"

	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
	"github.com/bluenviron/gomavlib/v3/pkg/message"

	"beta/internal/domain"
)

type recordingWriter struct {
	msgs []message.Message
	err  error
}

func (w *recordingWriter) WriteMessageAll(m message.Message) error {
	w.msgs = append(w.msgs, m)
	return w.err
}

func TestSendCommandInt(t *testing.T) {
	w := &recordingWriter{}
	err := NewSender(w).SendCommandInt(domain.CommandInt{SystemID: 3, Command: "DO_REPOSITION", Frame: "GLOBAL_RELATIVE_ALT_INT",
		Param1: -1, Param4: float32(math.NaN()), X: -353632610, Y: 1491652300, Z: 20})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := w.msgs[0].(*common.MessageCommandInt)
	if !ok {
		t.Fatalf("wrote %T, want *common.MessageCommandInt", w.msgs[0])
	}
	if m.TargetSystem != 3 || m.TargetComponent != uint8(common.MAV_COMP_ID_AUTOPILOT1) ||
		m.Command != common.MAV_CMD_DO_REPOSITION || m.Frame != common.MAV_FRAME_GLOBAL_RELATIVE_ALT_INT {
		t.Errorf("header fields = %+v", m)
	}
	if m.Param1 != -1 || !math.IsNaN(float64(m.Param4)) || m.X != -353632610 || m.Y != 1491652300 || m.Z != 20 {
		t.Errorf("params = %+v", m)
	}
}

func TestSendCommandIntBadNames(t *testing.T) {
	w := &recordingWriter{}
	s := NewSender(w)
	if err := s.SendCommandInt(domain.CommandInt{Command: "NOPE", Frame: "GLOBAL_RELATIVE_ALT_INT"}); err == nil {
		t.Error("unknown command: want an error")
	}
	if err := s.SendCommandInt(domain.CommandInt{Command: "DO_REPOSITION", Frame: "NOPE"}); err == nil {
		t.Error("unknown frame: want an error")
	}
	if len(w.msgs) != 0 {
		t.Errorf("wrote %d messages for bad names", len(w.msgs))
	}
	w.err = errors.New("link down")
	if err := s.SendCommandInt(domain.CommandInt{Command: "DO_REPOSITION", Frame: "GLOBAL_RELATIVE_ALT_INT"}); err == nil {
		t.Error("write error must come back")
	}
}
