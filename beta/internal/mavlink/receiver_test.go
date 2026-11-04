package mavlink

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"

	"beta/internal/app"
	"beta/internal/domain"
)

// recorder plays both the StateManager and the AckTracker.
type recorder struct {
	mu      sync.Mutex
	applied []any
	acks    []domain.CommandAckUpdate
}

func (r *recorder) Apply(update any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.applied = append(r.applied, update)
}

func (r *recorder) HandleAck(u domain.CommandAckUpdate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.acks = append(r.acks, u)
}

// runUntilClosed runs the receiver over the given events and fails (instead of hanging)
// if Run doesn't return after the channel is closed.
func runUntilClosed(t *testing.T, evts ...gomavlib.Event) *recorder {
	t.Helper()
	ch := make(chan gomavlib.Event, len(evts))
	for _, e := range evts {
		ch <- e
	}
	close(ch)

	rec := &recorder{}
	done := make(chan struct{})
	go func() {
		NewReceiver(ch, rec, rec).Run()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after the events channel was closed")
	}
	return rec
}

func TestReceiverRoutes(t *testing.T) {
	rec := runUntilClosed(t,
		&gomavlib.EventChannelOpen{},
		frameFrom(1, 1, &ardupilotmega.MessageHeartbeat{Type: ardupilotmega.MAV_TYPE_QUADROTOR}),
		&gomavlib.EventParseError{Error: errors.New("bad crc")},
		frameFrom(1, 1, &ardupilotmega.MessageAttitude{}),
		frameFrom(1, 1, &ardupilotmega.MessageCommandAck{
			Command: common.MAV_CMD_COMPONENT_ARM_DISARM, Result: ardupilotmega.MAV_RESULT_ACCEPTED,
		}),
		frameFrom(1, 1, &ardupilotmega.MessageGlobalPositionInt{Hdg: 65535}),
		&gomavlib.EventChannelClose{Error: errors.New("idle")},
	)

	if len(rec.acks) != 1 || rec.acks[0].Command != "COMPONENT_ARM_DISARM" {
		t.Errorf("acks = %+v, want one COMPONENT_ARM_DISARM", rec.acks)
	}
	if len(rec.applied) != 2 {
		t.Fatalf("applied %d updates, want 2 (heartbeat, position): %+v", len(rec.applied), rec.applied)
	}
	if _, ok := rec.applied[0].(domain.HeartbeatUpdate); !ok {
		t.Errorf("applied[0] is %T, want domain.HeartbeatUpdate", rec.applied[0])
	}
	if _, ok := rec.applied[1].(domain.GlobalPositionIntUpdate); !ok {
		t.Errorf("applied[1] is %T, want domain.GlobalPositionIntUpdate", rec.applied[1])
	}
}

// A parse error or channel event must not stop the loop: frames after it are still handled.
func TestReceiverKeepsGoingAfterErrors(t *testing.T) {
	rec := runUntilClosed(t,
		&gomavlib.EventParseError{Error: errors.New("bad crc")},
		&gomavlib.EventChannelClose{Error: errors.New("idle")},
		frameFrom(5, 1, &ardupilotmega.MessageHeartbeat{}),
	)
	if len(rec.applied) != 1 {
		t.Errorf("applied %d updates after errors, want 1", len(rec.applied))
	}
}

// The real StateManager and AckTracker must satisfy the Receiver's interfaces.
var (
	_ updateApplier = (*app.StateManager)(nil)
	_ ackHandler    = (*app.AckTracker)(nil)
)
