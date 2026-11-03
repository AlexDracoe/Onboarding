package app

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"beta/internal/domain"
)

const shortTimeout = 50 * time.Millisecond

func ack(systemID uint8, command, result string) domain.CommandAckUpdate {
	return domain.CommandAckUpdate{SystemID: systemID, Command: command, Result: result}
}

// returnsWithin fails the test if f doesn't return in time (catches a blocked HandleAck).
func returnsWithin(t *testing.T, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		f()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("%s did not return within 1s (blocked?)", what)
	}
}

// handleAck calls HandleAck but fails the test instead of hanging if it blocks.
func handleAck(t *testing.T, at *AckTracker, u domain.CommandAckUpdate) {
	t.Helper()
	returnsWithin(t, "HandleAck", func() { at.HandleAck(u) })
}

func TestRegisterTwiceIsAlreadyPending(t *testing.T) {
	at := NewAckTracker()
	p, err := at.Register(1, "COMPONENT_ARM_DISARM")
	if err != nil || p == nil {
		t.Fatalf("first Register: got (%v, %v), want a pending command and nil", p, err)
	}
	p2, err := at.Register(1, "COMPONENT_ARM_DISARM")
	if !errors.Is(err, ErrAlreadyPending) || p2 != nil {
		t.Fatalf("second Register: got (%v, %v), want (nil, ErrAlreadyPending)", p2, err)
	}
	// Different system or different command: independent keys.
	if _, err := at.Register(2, "COMPONENT_ARM_DISARM"); err != nil {
		t.Errorf("other system: %v", err)
	}
	if _, err := at.Register(1, "DO_SET_MODE"); err != nil {
		t.Errorf("other command: %v", err)
	}
}

func TestAckBeforeWaitStillDelivered(t *testing.T) {
	at := NewAckTracker()
	p, _ := at.Register(1, "COMPONENT_ARM_DISARM")
	returnsWithin(t, "HandleAck with nobody waiting yet", func() {
		at.HandleAck(ack(1, "COMPONENT_ARM_DISARM", "ACCEPTED"))
	})
	got, err := at.WaitForAck(p, shortTimeout)
	if err != nil || got != "ACCEPTED" {
		t.Fatalf("got (%q, %v), want (\"ACCEPTED\", nil)", got, err)
	}
}

func TestAckWhileWaiting(t *testing.T) {
	at := NewAckTracker()
	p, _ := at.Register(1, "NAV_TAKEOFF")
	go func() {
		time.Sleep(10 * time.Millisecond)
		at.HandleAck(ack(1, "NAV_TAKEOFF", "IN_PROGRESS"))
	}()
	got, err := at.WaitForAck(p, time.Second)
	if err != nil || got != "IN_PROGRESS" {
		t.Fatalf("got (%q, %v), want (\"IN_PROGRESS\", nil)", got, err)
	}
}

func TestTimeoutCleansUp(t *testing.T) {
	at := NewAckTracker()
	p, _ := at.Register(1, "COMPONENT_ARM_DISARM")
	got, err := at.WaitForAck(p, shortTimeout)
	if !errors.Is(err, ErrAckTimeout) || got != "" {
		t.Fatalf("got (%q, %v), want (\"\", ErrAckTimeout)", got, err)
	}
	if _, err := at.Register(1, "COMPONENT_ARM_DISARM"); err != nil {
		t.Fatalf("Register after timeout should work (entry cleaned up): %v", err)
	}
}

func TestResultClassification(t *testing.T) {
	cases := []struct {
		result        string
		wantOK        bool
		wantRetryable bool
	}{
		{"ACCEPTED", true, false},
		{"IN_PROGRESS", true, false},
		{"TEMPORARILY_REJECTED", false, true},
		{"DENIED", false, false},
		{"FAILED", false, false},
		{"UNSUPPORTED", false, false},
		{"CANCELLED", false, false},
	}
	for _, c := range cases {
		t.Run(c.result, func(t *testing.T) {
			at := NewAckTracker()
			p, _ := at.Register(3, "DO_SET_MODE")
			handleAck(t, at, ack(3, "DO_SET_MODE", c.result))
			got, err := at.WaitForAck(p, shortTimeout)
			if c.wantOK {
				if err != nil || got != c.result {
					t.Fatalf("got (%q, %v), want (%q, nil)", got, err, c.result)
				}
				return
			}
			var rej *CommandRejectedError
			if !errors.As(err, &rej) {
				t.Fatalf("got err %v (%T), want a *CommandRejectedError", err, err)
			}
			if rej.SystemID != 3 || rej.Command != "DO_SET_MODE" || rej.Result != c.result || rej.Retryable != c.wantRetryable {
				t.Errorf("got %+v, want system 3, DO_SET_MODE, %s, retryable=%t", *rej, c.result, c.wantRetryable)
			}
			if want := fmt.Sprintf("DO_SET_MODE rejected by system 3: %s", c.result); err.Error() != want {
				t.Errorf("message %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestDuplicateAcksNeverBlock(t *testing.T) {
	at := NewAckTracker()
	p, _ := at.Register(1, "NAV_TAKEOFF")
	returnsWithin(t, "three ACKs in a row", func() {
		at.HandleAck(ack(1, "NAV_TAKEOFF", "IN_PROGRESS"))
		at.HandleAck(ack(1, "NAV_TAKEOFF", "ACCEPTED"))
		at.HandleAck(ack(1, "NAV_TAKEOFF", "ACCEPTED"))
	})
	got, err := at.WaitForAck(p, shortTimeout)
	if err != nil || got != "IN_PROGRESS" {
		t.Fatalf("got (%q, %v), want the first ACK (\"IN_PROGRESS\", nil)", got, err)
	}
}

func TestUnexpectedAckIgnored(t *testing.T) {
	at := NewAckTracker()
	returnsWithin(t, "HandleAck with nothing registered", func() {
		at.HandleAck(ack(9, "COMPONENT_ARM_DISARM", "ACCEPTED"))
	})
}

func TestStaleDiscardKeepsRetry(t *testing.T) {
	at := NewAckTracker()
	first, _ := at.Register(1, "COMPONENT_ARM_DISARM")
	at.discard(first) // e.g. the send failed
	retry, err := at.Register(1, "COMPONENT_ARM_DISARM")
	if err != nil {
		t.Fatalf("retry Register: %v", err)
	}
	at.discard(first) // stale cleanup from the first attempt: must NOT remove the retry
	if _, err := at.Register(1, "COMPONENT_ARM_DISARM"); !errors.Is(err, ErrAlreadyPending) {
		t.Fatalf("retry entry was removed by a stale discard (Register got %v)", err)
	}
	handleAck(t, at, ack(1, "COMPONENT_ARM_DISARM", "ACCEPTED"))
	if got, err := at.WaitForAck(retry, shortTimeout); err != nil || got != "ACCEPTED" {
		t.Fatalf("retry wait: got (%q, %v)", got, err)
	}
}

func TestStaleTimeoutKeepsRetry(t *testing.T) {
	at := NewAckTracker()
	first, _ := at.Register(1, "DO_SET_MODE")
	handleAck(t, at, ack(1, "DO_SET_MODE", "ACCEPTED")) // consumes first's entry (option B)
	retry, err := at.Register(1, "DO_SET_MODE")         // a new attempt before the first waiter ran
	if err != nil {
		t.Fatalf("Register after the ACK consumed the entry: %v", err)
	}
	if _, err := at.WaitForAck(first, shortTimeout); err != nil {
		t.Fatalf("first wait: %v", err)
	}
	// first's deferred discard ran; the retry must still be registered.
	if _, err := at.Register(1, "DO_SET_MODE"); !errors.Is(err, ErrAlreadyPending) {
		t.Fatalf("first's cleanup removed the retry (Register got %v)", err)
	}
	_ = retry
}

func TestConcurrentSystems(t *testing.T) {
	at := NewAckTracker()
	commands := []string{"COMPONENT_ARM_DISARM", "DO_SET_MODE", "NAV_TAKEOFF", "DO_REPOSITION"}
	var wg sync.WaitGroup
	for sys := range uint8(20) {
		for _, cmd := range commands {
			wg.Go(func() {
				for range 20 {
					p, err := at.Register(sys, cmd)
					if err != nil {
						t.Errorf("Register(%d, %s): %v", sys, cmd, err)
						return
					}
					go at.HandleAck(ack(sys, cmd, "ACCEPTED"))
					if got, err := at.WaitForAck(p, time.Second); err != nil || got != "ACCEPTED" {
						t.Errorf("Wait(%d, %s): got (%q, %v)", sys, cmd, got, err)
						return
					}
				}
			})
		}
	}
	wg.Wait()
}
