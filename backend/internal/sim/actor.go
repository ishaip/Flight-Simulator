package sim

import (
	"flight-simulator/internal/clock"
	"flight-simulator/internal/env"
	"flight-simulator/internal/log"
	"fmt"
	"time"
)

// SimulationActor owns the AircraftState and advances it on every tick.
//
// Command ingestion follows the "finish-then-check" contract: after each tick
// is fully processed and the new state has been published, all pending commands
// on cmdCh are drained and the last one becomes active for the next tick.
// Commands never interrupt a tick mid-flight.
type SimulationActor struct {
	tickCh      <-chan clock.Tick
	cmdCh       chan Command
	store       *StateStore
	broadcaster *StateBroadcaster
	wind        *env.WindModel
	initial     AircraftState // saved initial state for Reset command
	logger      *log.Logger
}

// NewActor constructs a SimulationActor. Call Run in a goroutine.
func NewActor(
	tickCh <-chan clock.Tick,
	store *StateStore,
	broadcaster *StateBroadcaster,
	wind *env.WindModel,
	initial AircraftState,
	logger *log.Logger,
) *SimulationActor {
	store.Set(initial)
	return &SimulationActor{
		tickCh:      tickCh,
		cmdCh:       make(chan Command, 64),
		store:       store,
		broadcaster: broadcaster,
		wind:        wind,
		initial:     initial,
		logger:      logger,
	}
}

// SendCommand enqueues a command. It never blocks; if the buffer is full the
// oldest command is silently dropped (the newest wins).
func (a *SimulationActor) SendCommand(cmd Command) {
	select {
	case a.cmdCh <- cmd:
	default:
		// Buffer full — drain one stale entry and re-try.
		select {
		case <-a.cmdCh:
		default:
		}
		a.cmdCh <- cmd
	}
}

// Run starts the simulation loop. It returns when tickCh is closed (i.e. the
// ClockBus shuts down). Run is safe to call in its own goroutine.
func (a *SimulationActor) Run() {
	defer func() {
		if r := recover(); r != nil {
			msg := "Unexpected behavior: simulation crashed"
			if err, ok := r.(error); ok {
				msg = "Unexpected behavior: " + err.Error()
			}
			a.logger.Error(msg)
		}
	}()

	state := a.initial
	state.SimTime = time.Now().UTC()
	a.store.Set(state)
	a.broadcaster.Publish(state)

	var activeCmd Command // nil = stopped

	for tick := range a.tickCh {
		// --- 1. Advance physics ---
		dt := tick.DeltaT.Seconds()

		// Recover from panic in physics
		func() {
			defer func() {
				if r := recover(); r != nil {
					a.logger.Error("Physics calculation error")
				}
			}()
			state = Advance(state, activeCmd, a.wind, dt)
		}()

		state.SimTime = tick.SimTime
		state.Seq = tick.Seq

		// --- 2. Check for crash ---
		if a.checkCrash(state) {
			a.logger.Error("CRASH: Aircraft altitude reached 0 - plane crashed!")
			state.Alt = 0
			state.VLat = 0
			state.VLon = 0
			state.VAlt = 0
			activeCmd = Stop{}
		}

		// --- 2.5. Check for boundary violation ---
		if a.checkBoundaryViolation(state) {
			a.logger.Warning("Aircraft is accelerating too fast and leaving map boundaries!")
		}

		// --- 2.7. Log info if enabled ---
		a.logger.Info("State update: lat=" + fmt.Sprintf("%.6f", state.Lat) + ", lon=" + fmt.Sprintf("%.6f", state.Lon) + ", alt=" + fmt.Sprintf("%.1f", state.Alt))

		// --- 3. Publish new state ---
		a.store.Set(state)
		a.broadcaster.Publish(state)

		// --- 4. Finish-then-check: drain command channel ---
		// All commands queued since the last tick are consumed here; the last
		// one wins and becomes active on the NEXT tick.
		activeCmd = drainCmds(a.cmdCh, activeCmd)

		// --- 5. Handle Reset command specially ---
		if _, ok := activeCmd.(Reset); ok {
			state = a.initial
			state.SimTime = tick.SimTime
			state.Seq = tick.Seq
			a.store.Set(state)
			a.broadcaster.Publish(state)
			activeCmd = nil
		}
	}
}

// checkCrash returns true if the aircraft has crashed (altitude reached ground).
func (a *SimulationActor) checkCrash(state AircraftState) bool {
	return state.Alt <= 0 // Altitude at or below ground level
}

// checkBoundaryViolation returns true if the aircraft is leaving map boundaries at high speed.
func (a *SimulationActor) checkBoundaryViolation(state AircraftState) bool {
	// Map bounds: roughly ±10 degrees from Tel Aviv (32.0853, 34.7818)
	outOfBounds := state.Lat < 22 || state.Lat > 42 || state.Lon < 24 || state.Lon > 44
	if !outOfBounds {
		return false
	}

	// Check if accelerating too fast (combined velocity > 200 m/s)
	speed := (state.VLat * state.VLat) + (state.VLon * state.VLon) + (state.VAlt * state.VAlt)
	return speed > 40000 // sqrt(40000) = 200 m/s
}

// drainCmds reads every command currently in the channel without blocking.
// If no command is pending, current is returned unchanged.
func drainCmds(ch chan Command, current Command) Command {
	for {
		select {
		case cmd := <-ch:
			current = cmd
		default:
			return current
		}
	}
}
