package session

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"flight-simulator/internal/clock"
	"flight-simulator/internal/env"
	"flight-simulator/internal/log"
	"flight-simulator/internal/sim"
)

// Session represents a single user's flight simulator instance.
type Session struct {
	ID          string
	Store       *sim.StateStore
	Broadcaster *sim.StateBroadcaster
	Actor       *sim.SimulationActor
	Wind        *env.WindModel
	Logger      *log.Logger
	CreatedAt   time.Time
}

// Manager manages all active sessions.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	bus      *clock.ClockBus
}

// New creates a new session manager with a shared clock bus.
func New(bus *clock.ClockBus) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		bus:      bus,
	}
}

// GetOrCreate returns an existing session or creates a new one.
func (m *Manager) GetOrCreate(sessionID string) *Session {
	if sessionID == "" {
		sessionID = generateID()
	}

	m.mu.RLock()
	if s, ok := m.sessions[sessionID]; ok {
		m.mu.RUnlock()
		return s
	}
	m.mu.RUnlock()

	// Create new session
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if s, ok := m.sessions[sessionID]; ok {
		return s
	}

	// Create logger for this session
	logger, err := log.NewLogger(sessionID)
	if err != nil {
		logger = &log.Logger{} // Fallback (won't work well, but prevents panic)
	}

	// Initial state: Tel Aviv, 400m
	initial := sim.AircraftState{
		Lat:     32.0853,
		Lon:     34.7818,
		Alt:     400,
		Heading: 0,
		SimTime: time.Now().UTC(),
	}

	store := &sim.StateStore{}
	broadcaster := &sim.StateBroadcaster{}
	wind := env.New(0.000002, 0.000005, 0)
	tickCh := m.bus.Subscribe()
	actor := sim.NewActor(tickCh, store, broadcaster, wind, initial, logger)

	session := &Session{
		ID:          sessionID,
		Store:       store,
		Broadcaster: broadcaster,
		Actor:       actor,
		Wind:        wind,
		Logger:      logger,
		CreatedAt:   time.Now(),
	}

	m.sessions[sessionID] = session

	// Start actor goroutine
	go actor.Run()

	return session
}

// Get returns an existing session or nil if not found.
func (m *Manager) Get(sessionID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

// generateID creates a random session ID.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("session_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}
