// Package clock provides a ClockBus that delivers ticks to all subscribers at
// a configurable rate (default 20 Hz, range 10–60 Hz).
//
// Two modes:
//   - RealTime:    ticks at wall-clock intervals (1/hz). Delivery to each
//     subscriber is non-blocking; if a subscriber's buffer fills
//     past the warn threshold the tick is dropped and a warning is
//     printed — the wall-clock timer is never held back.
//   - FastForward: ticks with no sleep; simulated time advances at sim speed.
//     Delivery is still attempted non-blocking first, but if the
//     subscriber's buffer is too full the bus throttles itself with
//     a blocking send — there is no timer pressure so we can wait.
package clock

import (
	"log"
	"math"
	"sync"
	"time"
)

const (
	// subBufSize is the capacity of each subscriber's tick channel.
	subBufSize = 64
	// warnQueueLen is the pending-tick count at which a warning is printed.
	// Set at 75 % of subBufSize.
	warnQueueLen = subBufSize * 3 / 4
	// warnEvery suppresses repeated warnings; only every Nth overflow is logged.
	warnEvery = 20
)

// DefaultHz is the default tick rate.
const DefaultHz = 20.0

// MinHz / MaxHz bound the configurable rate.
const (
	MinHz = 10.0
	MaxHz = 60.0
)

// Tick is broadcast to every subscriber on every simulation step.
type Tick struct {
	// SimTime is the current simulated wall-clock time.
	SimTime time.Time
	// WallTime is the real wall-clock time at which the tick was sent.
	WallTime time.Time
	// DeltaT is the simulated duration of this step (= 1/Hz).
	DeltaT time.Duration
	// Seq is a monotonically increasing tick counter (starts at 1).
	Seq uint64
}

// ClockMode selects the tick delivery strategy.
type ClockMode int

const (
	RealTime    ClockMode = iota // ticks at wall-clock rate
	FastForward                  // ticks as fast as CPU allows
)

// cmdKind identifies internal command messages.
type cmdKind int

const (
	cmdSetHz cmdKind = iota
	cmdPause
	cmdResume
	cmdSkipBy
	cmdSkipTo
)

type clockCmd struct {
	kind cmdKind
	hz   float64
	d    time.Duration
	t    time.Time
}

// subscriber wraps a channel that receives ticks for one consumer.
type subscriber struct {
	ch        chan Tick
	overflows uint64 // total times the buffer was at or above warnQueueLen
}

// ClockBus manages the simulation clock and fans ticks out to subscribers.
type ClockBus struct {
	mu      sync.RWMutex
	subs    []*subscriber
	cmdCh   chan clockCmd
	resumeC chan struct{}
	// done is stored by Run so deliverTick can escape a blocking FF send on shutdown.
	done <-chan struct{}

	// current config — only read/written from the run goroutine
	hz      float64
	mode    ClockMode
	simTime time.Time
	simDt   time.Duration
	seq     uint64
	paused  bool

	// skip target, only valid in FastForward mode
	skipTarget time.Time
}

// New creates a ClockBus with the given starting simulated time and default Hz.
func New(startSimTime time.Time) *ClockBus {
	hz := DefaultHz
	return &ClockBus{
		hz:      hz,
		mode:    RealTime,
		simTime: startSimTime,
		simDt:   hzToDuration(hz),
		cmdCh:   make(chan clockCmd, 32),
		resumeC: make(chan struct{}, 1),
	}
}

// Subscribe returns a buffered channel that will receive Ticks.
// In RealTime mode ticks are dropped (with a warning) if the buffer fills;
// in FastForward mode the bus throttles itself instead of dropping.
func (b *ClockBus) Subscribe() <-chan Tick {
	sub := &subscriber{ch: make(chan Tick, subBufSize)}
	b.mu.Lock()
	b.subs = append(b.subs, sub)
	b.mu.Unlock()
	return sub.ch
}

// Unsubscribe removes the channel returned by Subscribe.
func (b *ClockBus) Unsubscribe(ch <-chan Tick) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, s := range b.subs {
		if s.ch == ch {
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			close(s.ch)
			return
		}
	}
}

// SetHz enqueues a clock-rate change. The new rate takes effect after the
// current tick's fan-out completes (finish-then-check).
func (b *ClockBus) SetHz(hz float64) {
	hz = clamp(hz, MinHz, MaxHz)
	b.cmdCh <- clockCmd{kind: cmdSetHz, hz: hz}
}

// Pause enqueues a pause. The clock stops after the current tick completes.
// User commands to the simulation actor are still accepted while paused.
func (b *ClockBus) Pause() { b.cmdCh <- clockCmd{kind: cmdPause} }

// Resume un-pauses the clock.
func (b *ClockBus) Resume() { b.cmdCh <- clockCmd{kind: cmdResume} }

// SkipBy enqueues a fast-forward of duration d in simulated time.
func (b *ClockBus) SkipBy(d time.Duration) { b.cmdCh <- clockCmd{kind: cmdSkipBy, d: d} }

// SkipTo enqueues a fast-forward until simulated time reaches t.
func (b *ClockBus) SkipTo(t time.Time) { b.cmdCh <- clockCmd{kind: cmdSkipTo, t: t} }

// Hz returns the current configured tick rate.
func (b *ClockBus) Hz() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.hz
}

// Mode returns the current clock mode.
func (b *ClockBus) Mode() ClockMode {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.mode
}

// Run starts the clock loop. It blocks until done is closed; call in a
// goroutine. On return all subscriber channels are closed.
func (b *ClockBus) Run(done <-chan struct{}) {
	b.done = done
	defer b.closeAllSubs()

	for {
		select {
		case <-done:
			return
		default:
		}

		if b.paused {
			// Block until Resume or shutdown.
			select {
			case <-b.resumeC:
				b.paused = false
			case <-done:
				return
			case cmd := <-b.cmdCh:
				b.applyCmd(cmd)
			}
			continue
		}

		switch b.mode {
		case RealTime:
			b.runRealTimeTick(done)
		case FastForward:
			b.runFastForwardTick(done)
		}
	}
}

// runRealTimeTick sleeps for one tick period, then fans out.
func (b *ClockBus) runRealTimeTick(done <-chan struct{}) {
	timer := time.NewTimer(b.simDt)
	defer timer.Stop()

	select {
	case <-done:
		return
	case <-timer.C:
	}

	b.fanOut()
	b.drainCmds()
}

// runFastForwardTick fans out immediately (no sleep) then checks for exit.
func (b *ClockBus) runFastForwardTick(done <-chan struct{}) {
	select {
	case <-done:
		return
	default:
	}

	b.fanOut()

	// Auto-revert once we've reached the skip target.
	if !b.simTime.Before(b.skipTarget) {
		b.mu.Lock()
		b.mode = RealTime
		b.mu.Unlock()
	}

	b.drainCmds()
}

// fanOut advances simTime, increments seq, and delivers a Tick to every
// subscriber via deliverTick.
func (b *ClockBus) fanOut() {
	b.seq++
	b.simTime = b.simTime.Add(b.simDt)

	tick := Tick{
		SimTime:  b.simTime,
		WallTime: time.Now(),
		DeltaT:   b.simDt,
		Seq:      b.seq,
	}

	b.mu.RLock()
	subs := make([]*subscriber, len(b.subs))
	copy(subs, b.subs)
	mode := b.mode
	b.mu.RUnlock()

	for _, s := range subs {
		b.deliverTick(s, tick, mode)
	}
}

// deliverTick sends tick to one subscriber using mode-appropriate strategy.
//
// RealTime: non-blocking send. If the subscriber's buffer is at or above
// warnQueueLen a warning is printed and the tick is dropped — the wall-clock
// timer must not be held back by a slow handler.
//
// FastForward: non-blocking attempt first. If the buffer is at or above
// warnQueueLen a warning is printed and the bus blocks until there is space
// (or shutdown fires). Since there is no timer in fast-forward mode the bus
// can afford to yield rather than lose ticks.
func (b *ClockBus) deliverTick(s *subscriber, tick Tick, mode ClockMode) {
	qlen := len(s.ch)

	if qlen >= warnQueueLen {
		s.overflows++
		if s.overflows == 1 || s.overflows%warnEvery == 0 {
			if mode == FastForward {
				log.Printf("⚠  clock: subscriber queue %d/%d (overflow #%d) — throttling fast-forward",
					qlen, subBufSize, s.overflows)
			} else {
				log.Printf("⚠  clock: subscriber queue %d/%d (overflow #%d) — dropping tick (real-time)",
					qlen, subBufSize, s.overflows)
			}
		}
		if mode == FastForward {
			// Throttle: block until the subscriber drains, but respect shutdown.
			select {
			case s.ch <- tick:
			case <-b.done:
			}
		}
		// RealTime: drop — return without sending.
		return
	}

	// Normal path: non-blocking send.
	select {
	case s.ch <- tick:
		if s.overflows > 0 {
			s.overflows = 0 // queue is flowing freely again
		}
	default:
		// Rare: buffer filled in the window between the len() check and select.
		s.overflows++
		if s.overflows%warnEvery == 0 {
			log.Printf("⚠  clock: subscriber queue full (race, overflow #%d)", s.overflows)
		}
		if mode == FastForward {
			select {
			case s.ch <- tick:
			case <-b.done:
			}
		}
	}
}

// drainCmds reads all pending commands without blocking, applying each one.
// This is the "finish-then-check" step executed after every tick fan-out.
func (b *ClockBus) drainCmds() {
	for {
		select {
		case cmd := <-b.cmdCh:
			b.applyCmd(cmd)
		default:
			return
		}
	}
}

// applyCmd applies one internal command.
func (b *ClockBus) applyCmd(cmd clockCmd) {
	switch cmd.kind {
	case cmdSetHz:
		b.mu.Lock()
		b.hz = cmd.hz
		b.simDt = hzToDuration(cmd.hz)
		b.mu.Unlock()

	case cmdPause:
		b.paused = true

	case cmdResume:
		// Drain the resume channel so the pause-select doesn't double-fire.
		select {
		case b.resumeC <- struct{}{}:
		default:
		}

	case cmdSkipBy:
		b.skipTarget = b.simTime.Add(cmd.d)
		b.mu.Lock()
		b.mode = FastForward
		b.mu.Unlock()

	case cmdSkipTo:
		b.skipTarget = cmd.t
		b.mu.Lock()
		b.mode = FastForward
		b.mu.Unlock()
	}
}

// closeAllSubs closes every subscriber channel so readers see EOF.
func (b *ClockBus) closeAllSubs() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range b.subs {
		close(s.ch)
	}
	b.subs = nil
}

// hzToDuration converts a frequency in Hz to a tick period.
func hzToDuration(hz float64) time.Duration {
	return time.Duration(math.Round(float64(time.Second) / hz))
}

// clamp returns v clamped to [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
