package sim

import (
	"flight-simulator/internal/clock"
	"flight-simulator/internal/env"
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
}

// NewActor constructs a SimulationActor. Call Run in a goroutine.
func NewActor(
	tickCh <-chan clock.Tick,
	store *StateStore,
	broadcaster *StateBroadcaster,
	wind *env.WindModel,
	initial AircraftState,
) *SimulationActor {
	store.Set(initial)
	return &SimulationActor{
		tickCh:      tickCh,
		cmdCh:       make(chan Command, 64),
		store:       store,
		broadcaster: broadcaster,
		wind:        wind,
		initial:     initial,
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
	state := a.store.Get()
	var activeCmd Command // nil = stopped

	for tick := range a.tickCh {
		// --- 1. Advance physics ---
		dt := tick.DeltaT.Seconds()
		state = Advance(state, activeCmd, a.wind, dt)
		state.SimTime = tick.SimTime
		state.Seq = tick.Seq

		// --- 2. Publish new state ---
		a.store.Set(state)
		a.broadcaster.Publish(state)

		// --- 3. Finish-then-check: drain command channel ---
		// All commands queued since the last tick are consumed here; the last
		// one wins and becomes active on the NEXT tick.
		activeCmd = drainCmds(a.cmdCh, activeCmd)

		// --- 4. Handle Reset command specially ---
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
