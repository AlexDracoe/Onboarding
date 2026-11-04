package mavlink

import (
	"log"

	"github.com/bluenviron/gomavlib/v3"

	"beta/internal/domain"
)

// updateApplier is what the Receiver needs from the StateManager.
type updateApplier interface {
	Apply(update any)
}

// ackHandler is what the Receiver needs from the AckTracker.
type ackHandler interface {
	HandleAck(u domain.CommandAckUpdate)
}

// Receiver is the single inbound consumer: ACKs go to the AckTracker, telemetry to the StateManager.
type Receiver struct {
	events <-chan gomavlib.Event
	state  updateApplier
	acks   ackHandler
}

func NewReceiver(events <-chan gomavlib.Event, state updateApplier, acks ackHandler) *Receiver {
	return &Receiver{events: events, state: state, acks: acks}
}

// Run blocks until the events channel is closed (node.Close). Start it with `go r.Run()`.
func (r *Receiver) Run() {
	for evt := range r.events {
		switch e := evt.(type) {
		case *gomavlib.EventFrame:
			r.handleFrame(e)
		case *gomavlib.EventChannelOpen:
			log.Printf("mavlink: channel open: %s", e.Channel)
		case *gomavlib.EventChannelClose:
			log.Printf("mavlink: channel closed: %s: %v", e.Channel, e.Error)
		case *gomavlib.EventParseError:
			log.Printf("mavlink: parse error: %v", e.Error)
		}
	}
	log.Print("mavlink: receiver stopped")
}

func (r *Receiver) handleFrame(frm *gomavlib.EventFrame) {
	switch u := mapFrame(frm).(type) {
	case domain.CommandAckUpdate:
		r.acks.HandleAck(u)
	case nil:
		// Unmapped message (ATTITUDE, SYS_STATUS, ...): SITL sends dozens per second, so no log.
	default:
		r.state.Apply(u)
	}
}
