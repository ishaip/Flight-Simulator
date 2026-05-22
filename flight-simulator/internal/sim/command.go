package sim

// Command is implemented by every instruction the pilot can issue.
type Command interface {
	commandTag() // sealed interface — only this package's types satisfy it
}

// GotoPoint commands the aircraft to fly toward a geographic position.
type GotoPoint struct {
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Alt   float64 `json:"alt"`
	Speed float64 `json:"speed"` // target ground speed in m/s; 0 = keep current
}

func (GotoPoint) commandTag() {}

// Waypoint is a single step in a Trajectory.
type Waypoint struct {
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Alt   float64 `json:"alt"`
	Speed float64 `json:"speed"` // 0 = keep current
}

// Trajectory commands the aircraft to follow an ordered list of waypoints.
type Trajectory struct {
	Waypoints []Waypoint `json:"waypoints"`
	Loop      bool       `json:"loop"`
	// currentIdx tracks which waypoint we are flying toward (not exposed to API).
	currentIdx int
}

func (t *Trajectory) commandTag() {}

// CurrentWaypoint returns the active waypoint or nil if the trajectory is done.
func (t *Trajectory) CurrentWaypoint() *Waypoint {
	if t.currentIdx >= len(t.Waypoints) {
		return nil
	}
	return &t.Waypoints[t.currentIdx]
}

// Advance moves to the next waypoint; wraps around when Loop is set.
// Returns true if there is another waypoint to fly to.
func (t *Trajectory) Advance() bool {
	t.currentIdx++
	if t.currentIdx >= len(t.Waypoints) {
		if t.Loop {
			t.currentIdx = 0
			return true
		}
		return false
	}
	return true
}

// Stop commands the aircraft to cease movement and clear the active command.
type Stop struct{}

func (Stop) commandTag() {}

// Hold commands the aircraft to maintain its current position (zero velocity).
type Hold struct{}

func (Hold) commandTag() {}

// Accelerate commands a direct throttle input. Value is in m/s² along the
// current heading; negative values decelerate.
type Accelerate struct {
	Value float64 `json:"value"`
}

func (Accelerate) commandTag() {}

// Reset resets the aircraft to its initial position and state.
type Reset struct{}

func (Reset) commandTag() {}
