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
}

func (t *Trajectory) commandTag() {}

// CurrentWaypoint returns the active waypoint or nil if the trajectory is done.
func (t *Trajectory) CurrentWaypoint() *Waypoint {
	if len(t.Waypoints) == 0 {
		return nil
	}
	return &t.Waypoints[0]
}

// Advance removes the current waypoint and moves to the next one.
// Returns true if there is another waypoint to fly to after removal.
func (t *Trajectory) Advance() bool {
	if len(t.Waypoints) > 0 {
		// Remove the first (current) waypoint
		t.Waypoints = t.Waypoints[1:]
	}
	if len(t.Waypoints) == 0 {
		if t.Loop {
			// For looping trajectories, this would need the original waypoints
			// For now, just signal that trajectory is done
			return false
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

// SetHeading sets the aircraft's heading (direction to fly) without affecting velocity.
type SetHeading struct {
	Heading float64 `json:"heading"` // degrees, 0 = north, 90 = east, 180 = south, 270 = west
}

func (SetHeading) commandTag() {}

// Reset resets the aircraft to its initial position and state.
type Reset struct{}

func (Reset) commandTag() {}
