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
	ID            string
	PlaneType     sim.PlaneType
	Store         *sim.StateStore
	Broadcaster   *sim.StateBroadcaster
	LogBroadcaster *log.LogBroadcaster
	Actor         *sim.SimulationActor
	Wind          *env.WindModel
	Logger        *log.Logger
	CreatedAt     time.Time
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
// If planeType is empty, defaults to Cessna.
func (m *Manager) GetOrCreate(sessionID string, planeType sim.PlaneType) *Session {
	if sessionID == "" {
		sessionID = generateID()
	}
	if planeType == "" {
		planeType = sim.PlaneCessna
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

	// Create log broadcaster
	logBroadcaster := &log.LogBroadcaster{}
	logger.SetLogBroadcaster(logBroadcaster)

	// Initial state: Tel Aviv, 400m, starting speed based on plane type
	planeProps := sim.PlaneProperties[planeType]
	initialSpeedDegPerSec := planeProps.CruiseSpeedMS / 111_320.0

	initial := sim.AircraftState{
		Lat:       32.0853,
		Lon:       34.7818,
		Alt:       400,
		Heading:   0,
		PlaneType: planeType,
		VLat:      initialSpeedDegPerSec, // Initial heading north
		VLon:      0,                     // no east-west velocity
		VAlt:      0,                     // no vertical velocity
		SimTime:   time.Now().UTC(),
	}

	store := &sim.StateStore{}
	broadcaster := &sim.StateBroadcaster{}
	wind := env.New(0.000002, 0.000005, 0)
	tickCh := m.bus.Subscribe()
	actor := sim.NewActor(tickCh, store, broadcaster, wind, initial, logger)

	session := &Session{
		ID:            sessionID,
		PlaneType:     planeType,
		Store:         store,
		Broadcaster:   broadcaster,
		LogBroadcaster: logBroadcaster,
		Actor:         actor,
		Wind:          wind,
		Logger:        logger,
		CreatedAt:     time.Now(),
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
