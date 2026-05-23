package sim

import (
	"math"

	"flight-simulator/internal/env"
)

const (
	// maxClimbRateMS is the maximum vertical speed in m/s.
	maxClimbRateMS = 20.0

	// arrivalToleranceDeg is the arrival detection radius in degrees.
	arrivalToleranceDeg = 0.0001 // ~11 m

	// metersPerDegree is a rough conversion at mid-latitudes.
	metersPerDegree = 111_320.0

	// maxSpeedDegPerSec is the maximum horizontal ground speed in degrees/s
	// (clamping limit to prevent numerical issues)
	maxSpeedDegPerSec = 0.01
)

// Advance computes the next AircraftState given the current state, active
// command, wind, and the simulated time-step dt in seconds.
func Advance(s AircraftState, cmd Command, wind *env.WindModel, dt float64) AircraftState {
	wLat, wLon, wAlt := wind.Vector()

	switch c := cmd.(type) {
	case GotoPoint:
		return advanceGoto(s, c, wLat, wLon, wAlt, dt)
	case *Trajectory:
		return advanceTrajectory(s, c, wLat, wLon, wAlt, dt)
	case SetHeading:
		return advanceSetHeading(s, c, wLat, wLon, wAlt, dt)
	case Hold:
		return advanceHold(s, wLat, wLon, wAlt, dt)
	case Stop:
		return advanceStop(s)
	case nil:
		// No active command: continue cruising with current velocity
		return advanceCruise(s, wLat, wLon, wAlt, dt)
	case Reset:
		// Reset will be handled at a higher level in Actor.Run()
		return s
	default:
		return s
	}
}

// advanceGoto steers toward the target point.
func advanceGoto(s AircraftState, g GotoPoint, wLat, wLon, wAlt, dt float64) AircraftState {
	// Get plane-specific cruise speed
	planeProps := PlaneProperties[s.PlaneType]
	desiredSpeed := g.Speed / metersPerDegree // convert m/s → °/s for horizontal
	if desiredSpeed <= 0 {
		desiredSpeed = planeProps.CruiseSpeedMS / metersPerDegree
	}

	bearing := bearingDeg(s.Lat, s.Lon, g.Lat, g.Lon)
	s.Heading = bearing // Set heading directly (instant turn, no rate limit)

	dLat := g.Lat - s.Lat
	dLon := g.Lon - s.Lon
	distDeg := math.Sqrt(dLat*dLat + dLon*dLon)

	if distDeg < arrivalToleranceDeg {
		// Arrived — hold position.
		s.VLat, s.VLon = 0, 0
		s.Lat, s.Lon = g.Lat, g.Lon
	} else {
		rad := s.Heading * math.Pi / 180
		s.VLat = desiredSpeed * math.Cos(rad)
		s.VLon = desiredSpeed * math.Sin(rad)

		// Apply wind with plane-specific resistance
		scaledWLat := wLat * planeProps.WindResistance
		scaledWLon := wLon * planeProps.WindResistance

		// Apply wind and clamp.
		s.VLat = clamp(s.VLat+scaledWLat, -maxSpeedDegPerSec, maxSpeedDegPerSec)
		s.VLon = clamp(s.VLon+scaledWLon, -maxSpeedDegPerSec, maxSpeedDegPerSec)

		s.Lat += s.VLat * dt
		s.Lon += s.VLon * dt
	}

	// Altitude.
	dAlt := g.Alt - s.Alt
	climbRate := math.Copysign(math.Min(math.Abs(dAlt/dt), maxClimbRateMS), dAlt)
	s.VAlt = clamp(climbRate+wAlt, -maxClimbRateMS, maxClimbRateMS)
	s.Alt += s.VAlt * dt
	if math.Abs(g.Alt-s.Alt) < 1.0 {
		s.Alt = g.Alt
		s.VAlt = 0
	}

	return s
}

// advanceTrajectory delegates to GotoPoint logic for the current waypoint.
func advanceTrajectory(s AircraftState, t *Trajectory, wLat, wLon, wAlt, dt float64) AircraftState {
	wp := t.CurrentWaypoint()
	if wp == nil {
		return advanceStop(s)
	}

	g := GotoPoint{Lat: wp.Lat, Lon: wp.Lon, Alt: wp.Alt, Speed: wp.Speed}
	next := advanceGoto(s, g, wLat, wLon, wAlt, dt)

	// Check arrival: if velocity went to zero at the target, advance waypoint.
	// Only advance if both horizontal and vertical distances are small.
	if next.VLat == 0 && next.VLon == 0 && next.VAlt == 0 {
		dLat := next.Lat - wp.Lat
		dLon := next.Lon - wp.Lon
		dAlt := next.Alt - wp.Alt
		horizDist := math.Sqrt(dLat*dLat + dLon*dLon)
		
		// Arrival if both horizontal (< tolerance) and vertical (< 1m) distances are met
		if horizDist < arrivalToleranceDeg && math.Abs(dAlt) < 1.0 {
			t.Advance()
		}
	}
	return next
}

// advanceSetHeading sets the aircraft heading without affecting velocity.
func advanceSetHeading(s AircraftState, sh SetHeading, wLat, wLon, wAlt, dt float64) AircraftState {
	s.Heading = sh.Heading

	// Get plane-specific wind resistance
	planeProps := PlaneProperties[s.PlaneType]
	scaledWLat := wLat * planeProps.WindResistance
	scaledWLon := wLon * planeProps.WindResistance

	// Apply wind and clamp velocity
	s.VLat = clamp(s.VLat+scaledWLat, -maxSpeedDegPerSec, maxSpeedDegPerSec)
	s.VLon = clamp(s.VLon+scaledWLon, -maxSpeedDegPerSec, maxSpeedDegPerSec)
	s.VAlt = clamp(s.VAlt+wAlt, -maxClimbRateMS, maxClimbRateMS)

	// Update position based on velocity
	s.Lat += s.VLat * dt
	s.Lon += s.VLon * dt
	s.Alt += s.VAlt * dt

	return s
}

// advanceCruise continues cruising with current velocity (no command).
// Applies wind and updates position without changing velocity.
func advanceCruise(s AircraftState, wLat, wLon, wAlt, dt float64) AircraftState {
	// Get plane-specific wind resistance
	planeProps := PlaneProperties[s.PlaneType]
	scaledWLat := wLat * planeProps.WindResistance
	scaledWLon := wLon * planeProps.WindResistance

	// Apply wind effect and clamp velocity
	s.VLat = clamp(s.VLat+scaledWLat, -maxSpeedDegPerSec, maxSpeedDegPerSec)
	s.VLon = clamp(s.VLon+scaledWLon, -maxSpeedDegPerSec, maxSpeedDegPerSec)
	s.VAlt = clamp(s.VAlt+wAlt, -maxClimbRateMS, maxClimbRateMS)

	// Update position based on velocity
	s.Lat += s.VLat * dt
	s.Lon += s.VLon * dt
	s.Alt += s.VAlt * dt

	return s
}

// advanceHold zeroes velocity and holds position against wind.
func advanceHold(s AircraftState, wLat, wLon, wAlt, dt float64) AircraftState {
	// Active station-keeping: apply anti-wind each tick.
	s.VLat = -wLat
	s.VLon = -wLon
	s.VAlt = -wAlt
	s.Lat += s.VLat * dt
	s.Lon += s.VLon * dt
	s.Alt += s.VAlt * dt
	return s
}

// advanceStop zeroes velocity without moving.
func advanceStop(s AircraftState) AircraftState {
	s.VLat = 0
	s.VLon = 0
	s.VAlt = 0
	return s
}

// bearingDeg returns the initial bearing in degrees from (lat1,lon1) to
// (lat2,lon2) using the great-circle formula.
func bearingDeg(lat1, lon1, lat2, lon2 float64) float64 {
	la1 := lat1 * math.Pi / 180
	la2 := lat2 * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180

	y := math.Sin(dLon) * math.Cos(la2)
	x := math.Cos(la1)*math.Sin(la2) - math.Sin(la1)*math.Cos(la2)*math.Cos(dLon)
	return math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
}

// clamp constrains v to [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
