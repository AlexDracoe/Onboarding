package app

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"beta/internal/domain"
)

var (
	ErrAlreadyPending = errors.New("command already pending")
	ErrAckTimeout     = errors.New("no ACK before timeout")
)

// CommandRejectedError means the vehicle answered but not with a success result
type CommandRejectedError struct {
	SystemID  uint8
	Command   string
	Result    string
	Retryable bool
}

func (e *CommandRejectedError) Error() string {
	return fmt.Sprintf("%s rejected by system %d: %s", e.Command, e.SystemID, e.Result)

}

// PendingCommand is one sent command waiting for its COMMAND_ACK
type PendingCommand struct {
	systemID uint8
	command  string
	result   chan string //Cap 1
}
type key struct {
	systemID uint8
	command  string
}

// ACKTracker matches incoming COMMAND_ACK to the command waiting for them
// At most one pending command per (systemID command name)
type AckTracker struct {
	pending map[key]*PendingCommand
	mu      sync.Mutex
}

func NewAckTracker() *AckTracker {
	return &AckTracker{pending: make(map[key]*PendingCommand)}
}

// Register records a command before it is sent
func (at *AckTracker) Register(systemID uint8, command string) (*PendingCommand, error) {
	k := key{systemID: systemID, command: command}
	p := &PendingCommand{systemID: systemID, command: command, result: make(chan string, 1)}

	at.mu.Lock()
	defer at.mu.Unlock()
	if _, ok := at.pending[k]; ok {
		return nil, fmt.Errorf("%s for system %d: %w", command, systemID, ErrAlreadyPending)
	}
	at.pending[k] = p
	return p, nil
}

// HandleAck is called by the receiver goroutine for every COMMAND_ACK it never blocks
func (at *AckTracker) HandleAck(update domain.CommandAckUpdate) {
	k := key{systemID: update.SystemID, command: update.Command}

	at.mu.Lock()
	defer at.mu.Unlock()
	p, ok := at.pending[k]
	if !ok {
		log.Printf("unexpected ACK: system %d %s %s", update.SystemID, update.Command, update.Result)
		return
	}

	p.result <- update.Result
	delete(at.pending, k)
}

// WaitForAck blocks until the ACK arrives or timeout passes It cleans up on every path
func (at *AckTracker) WaitForAck(p *PendingCommand, timeout time.Duration) (string, error) {
	defer at.discard(p)

	var result string
	select {
	case result = <-p.result:
	case <-time.After(timeout):
		return "", fmt.Errorf("%s for system %d: %w", p.command, p.systemID, ErrAckTimeout)
	}

	switch result {
	case "ACCEPTED", "IN_PROGRESS":
		return result, nil
	}
	return "", &CommandRejectedError{
		SystemID:  p.systemID,
		Command:   p.command,
		Result:    result,
		Retryable: result == "TEMPORARILY_REJECTED",
	}

}

// discard removes p, but only if p
func (at *AckTracker) discard(p *PendingCommand) {
	k := key{systemID: p.systemID, command: p.command}

	at.mu.Lock()
	defer at.mu.Unlock()
	if at.pending[k] == p {
		delete(at.pending, k)
	}
}
