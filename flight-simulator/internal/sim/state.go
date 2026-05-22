package sim

import (
	"sync"
	"time"
)

// AircraftState is the full snapshot of the aircraft at one simulation step.
// Velocity components are in degrees/second for lat & lon, and meters/second
// for altitude, which keeps the stats display natural.
type AircraftState struct {
	// Position
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	Alt float64 `json:"alt"` // metres

	// Velocity (degrees/s for lat/lon, m/s for alt)
	VLat float64 `json:"vLat"`
	VLon float64 `json:"vLon"`
	VAlt float64 `json:"vAlt"`

	// Heading in degrees, 0 = North, clockwise.
	Heading float64 `json:"heading"`

	// SimTime is the simulated timestamp for this state.
	SimTime time.Time `json:"simTime"`

	// Seq is the tick sequence number that produced this state.
	Seq uint64 `json:"seq"`
}

// StateStore is a thread-safe container that always holds the latest state.
type StateStore struct {
	mu    sync.RWMutex
	state AircraftState
}

// Set replaces the stored state.
func (s *StateStore) Set(st AircraftState) {
	s.mu.Lock()
	s.state = st
	s.mu.Unlock()
}

// Get returns a copy of the latest state.
func (s *StateStore) Get() AircraftState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// StateBroadcaster fans the latest state out to any number of SSE listeners.
// Subscribe returns a channel; Unsubscribe closes and removes it.
type StateBroadcaster struct {
	mu   sync.Mutex
	subs []chan AircraftState
}

// Subscribe registers a new listener and returns its channel.
func (b *StateBroadcaster) Subscribe() <-chan AircraftState {
	ch := make(chan AircraftState, 4)
	b.mu.Lock()
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes the channel previously returned by Subscribe.
func (b *StateBroadcaster) Unsubscribe(ch <-chan AircraftState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, s := range b.subs {
		if s == ch {
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			close(s)
			return
		}
	}
}

// Publish sends state to every subscriber. Sends are non-blocking; if a
// subscriber's buffer is full the update is dropped for that subscriber only.
func (b *StateBroadcaster) Publish(st AircraftState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- st:
		default:
		}
	}
}
