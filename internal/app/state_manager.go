package app

import (
	"fmt"
	"sync"

	"beta/internal/domain"
)

type StateManager struct {
	mu       sync.Mutex
	vehicles map[uint8]*domain.VehicleState
}

func NewStateManager() *StateManager {
	return &StateManager{vehicles: make(map[uint8]*domain.VehicleState)}
}

func (sm *StateManager) Apply(update any) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	switch currentUpdate := update.(type) {
	case domain.HeartbeatUpdate:
		sm.applyHeartbeat(currentUpdate)
	case domain.GlobalPositionIntUpdate:
		sm.applyGlobalPosition(currentUpdate)
	default:
		log.Printf(... %T ..., update)
	}
}

// applyHeartbeat assumes sm.mu is held. It does NOT lock (non-reentrant mutex).
func (sm *StateManager) applyHeartbeat(u domain.HeartbeatUpdate) {
	// TODO: v := sm.vehicleLocked(u.SystemID)
	// TODO: assign every heartbeat field as a NEW pointer (new(...)); never *v.X = ...
	// TODO: bump the heartbeat counter
	// TODO: record the heartbeat time

}

// applyGlobalPosition assumes sm.mu is held.
func (sm *StateManager) applyGlobalPosition(u domain.GlobalPositionIntUpdate) {
	// TODO: v := sm.vehicleLocked(u.SystemID)
	// TODO: point Position / Velocity at NEW values built from u (u is already your own copy)
	// TODO: heading: u.Heading is already a pointer (or nil); can you just assign it?
	// TODO: record the position time

}

// vehicleLocked returns the stored vehicle for id creating it on first sight. Assumes sm.mu is held
func (sm *StateManager) vehicleLocked(id uint8) *domain.VehicleState {
	vehiclePtr, ok := sm.vehicles[id]

	if !ok {

	}
}

func (sm *StateManager) Get(id uint8) (domain.VehicleState, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	vehiclePtr, ok := sm.vehicles[id]
	if !ok {
		return domain.VehicleState{}, false
	} 
	
	return *vehiclePtr, true


}

func (sm *StateManager) List() []domain.VehicleState {

}
