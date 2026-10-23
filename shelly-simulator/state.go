package main

import (
	"sync"
	"time"
)

// State is the single shared source of truth for the simulated plug,
// read/written by Gen1 HTTP, Gen2 HTTP/WS RPC, and MQTT handlers alike.
type State struct {
	mu sync.Mutex

	deviceID    string
	authEnabled bool
	on          bool
	powerW      float64 // instantaneous simulated power draw
	energyWh    float64 // accumulated energy
	startedAt   time.Time
}

func newState(deviceID string, authEnabled bool) *State {
	return &State{deviceID: deviceID, authEnabled: authEnabled, startedAt: time.Now()}
}

// SetRelay sets the relay on/off and returns the new state plus whether it changed.
func (s *State) SetRelay(on bool) (changed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed = s.on != on
	s.on = on
	if !on {
		s.powerW = 0
	}
	return changed
}

func (s *State) Toggle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.on = !s.on
	if !s.on {
		s.powerW = 0
	}
	return s.on
}

func (s *State) Snapshot() (on bool, powerW, energyWh float64, uptime time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.on, s.powerW, s.energyWh, time.Since(s.startedAt)
}

// tick simulates power draw and energy accumulation for the given interval.
func (s *State) tick(interval time.Duration) (changed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.on {
		return false
	}
	// ponytail: naive sim, fixed baseline + tiny jitter, swap for a real load model if fidelity matters
	s.powerW = 50 + float64(time.Now().UnixNano()%2000)/100
	s.energyWh += s.powerW * interval.Hours()
	return true
}
