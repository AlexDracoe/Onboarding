package app

import (
	"log"
	"maps"
	"slices"
	"sync"
	"time"

	"beta/internal/domain"
)

// StateManager holds the latest known state of every vehicle.
// One mutex guards the map; every public method locks it.
// Rule: never write through a stored pointer, always assign a new one (Get/List copies share pointees).
type StateManager struct {
	mu       sync.Mutex
	vehicles map[uint8]*domain.VehicleState
}

// NewStateManager returns an empty, ready-to-use StateManager.
func NewStateManager() *StateManager {
	return &StateManager{vehicles: make(map[uint8]*domain.VehicleState)}
}

// Apply folds one telemetry update into the vehicle's state. Unknown update types are logged and ignored.
func (sm *StateManager) Apply(update any) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	switch currentUpdate := update.(type) {
	case domain.HeartbeatUpdate:
		sm.applyHeartbeat(currentUpdate)
	case domain.GlobalPositionIntUpdate:
		sm.applyGlobalPosition(currentUpdate)
	default:
		log.Printf("state: ignoring update of type %T", update)
	}
}

// applyHeartbeat assumes sm.mu is held. It does NOT lock (non-reentrant mutex).
func (sm *StateManager) applyHeartbeat(u domain.HeartbeatUpdate) {
	v := sm.vehicleLocked(u.SystemID)

	v.ComponentID = new(u.ComponentID)
	v.VehicleType = new(u.VehicleType)
	v.Autopilot = new(u.Autopilot)
	v.IsArmed = new(u.IsArmed)
	v.FlightMode = new(u.FlightMode)
	v.SystemStatus = new(u.SystemStatus)

	v.HeartbeatCount++
	v.LastHeartbeatUpdateTime = time.Now()
}

// applyGlobalPosition assumes sm.mu is held.
func (sm *StateManager) applyGlobalPosition(u domain.GlobalPositionIntUpdate) {
	v := sm.vehicleLocked(u.SystemID)
	v.Position = new(u.Position)
	v.Velocity = new(u.Velocity)
	v.Heading = u.Heading

	v.LastGlobalPositionUpdateTime = time.Now()
}

// vehicleLocked returns the stored vehicle for id, creating it on first sight. Assumes sm.mu is held.
func (sm *StateManager) vehicleLocked(id uint8) *domain.VehicleState {
	vehiclePtr, ok := sm.vehicles[id]
	if !ok {
		vehiclePtr = &domain.VehicleState{SystemID: id}
		sm.vehicles[id] = vehiclePtr
	}
	return vehiclePtr
}

// Get returns a copy of the vehicle's state, or ok=false if nothing has been heard from it.
func (sm *StateManager) Get(id uint8) (domain.VehicleState, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	vehiclePtr, ok := sm.vehicles[id]
	if !ok {
		return domain.VehicleState{}, false
	}

	return *vehiclePtr, true
}

// List returns copies of all known vehicles, sorted by system ID.
func (sm *StateManager) List() []domain.VehicleState {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	out := make([]domain.VehicleState, 0, len(sm.vehicles))
	for _, k := range slices.Sorted(maps.Keys(sm.vehicles)) {
		out = append(out, *sm.vehicles[k])
	}
	return out
}
