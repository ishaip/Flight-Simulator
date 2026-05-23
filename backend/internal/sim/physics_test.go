package sim

import (
	"math"
	"testing"

	"flight-simulator/internal/env"
)

// ---- helpers ----------------------------------------------------------------

func noWind() *env.WindModel { return env.New(0, 0, 0) }

func enabledWind(vLat, vLon, vAlt float64) *env.WindModel {
	w := env.New(vLat, vLon, vAlt)
	w.SetEnabled(true)
	return w
}

func cessnaAt(lat, lon, alt float64) AircraftState {
	return AircraftState{PlaneType: PlaneCessna, Lat: lat, Lon: lon, Alt: alt}
}

// ---- Advance dispatcher -----------------------------------------------------

func TestAdvance_Stop_ZeroesVelocityNoPositionChange(t *testing.T) {
	s := cessnaAt(10, 20, 500)
	s.VLat, s.VLon, s.VAlt = 0.001, 0.002, 5.0

	got := Advance(s, Stop{}, noWind(), 1.0)

	if got.VLat != 0 || got.VLon != 0 || got.VAlt != 0 {
		t.Errorf("Stop: want zero velocity, got (%.6f, %.6f, %.6f)", got.VLat, got.VLon, got.VAlt)
	}
	if got.Lat != s.Lat || got.Lon != s.Lon || got.Alt != s.Alt {
		t.Errorf("Stop: position must not change: got (%.6f, %.6f, %.1f)", got.Lat, got.Lon, got.Alt)
	}
}

func TestAdvance_NilCommand_CruisesWithCurrentVelocity(t *testing.T) {
	s := cessnaAt(0, 0, 1000)
	s.VLat = 0.001
	dt := 1.0

	got := Advance(s, nil, noWind(), dt)

	wantLat := s.Lat + s.VLat*dt
	if math.Abs(got.Lat-wantLat) > 1e-9 {
		t.Errorf("NilCruise: lat = %.9f, want %.9f", got.Lat, wantLat)
	}
	if math.Abs(got.Lon) > 1e-9 {
		t.Errorf("NilCruise: lon should stay ~0, got %.9f", got.Lon)
	}
}

func TestAdvance_Reset_ReturnsStateUnchanged(t *testing.T) {
	s := cessnaAt(3, 4, 200)
	got := Advance(s, Reset{}, noWind(), 1.0)
	if got.Lat != s.Lat || got.Lon != s.Lon {
		t.Errorf("Reset: Advance must not move the aircraft, got (%.6f, %.6f)", got.Lat, got.Lon)
	}
}

// ---- GotoPoint --------------------------------------------------------------

func TestAdvance_Goto_MovesNorthTowardTarget(t *testing.T) {
	s := cessnaAt(0, 0, 1000)
	cmd := GotoPoint{Lat: 1.0, Lon: 0.0, Alt: 1000.0}

	got := Advance(s, cmd, noWind(), 1.0)

	if got.Lat <= s.Lat {
		t.Errorf("Goto north: lat should increase, got %.6f (was %.6f)", got.Lat, s.Lat)
	}
	if math.Abs(got.Lon) > 1e-9 {
		t.Errorf("Goto north: lon should stay ~0, got %.9f", got.Lon)
	}
}

func TestAdvance_Goto_SnapsAndZerosVelocityOnArrival(t *testing.T) {
	// Aircraft already at target (within arrival tolerance).
	s := cessnaAt(1.0, 1.0, 500)
	s.VLat, s.VLon = 0.005, 0.005
	cmd := GotoPoint{Lat: 1.0, Lon: 1.0, Alt: 500.0}

	got := Advance(s, cmd, noWind(), 1.0)

	if got.VLat != 0 || got.VLon != 0 {
		t.Errorf("Goto arrival: want zero horizontal velocity, got (%.6f, %.6f)", got.VLat, got.VLon)
	}
	if got.Lat != 1.0 || got.Lon != 1.0 {
		t.Errorf("Goto arrival: position should snap to target, got (%.6f, %.6f)", got.Lat, got.Lon)
	}
}

func TestAdvance_Goto_ClimbsTowardAltitude(t *testing.T) {
	s := cessnaAt(0, 0, 0)
	cmd := GotoPoint{Lat: 10.0, Lon: 0.0, Alt: 1000.0}

	got := Advance(s, cmd, noWind(), 1.0)

	if got.Alt <= 0 {
		t.Errorf("Goto: altitude should increase toward 1000 m, got %.1f", got.Alt)
	}
}

func TestAdvance_Goto_AltitudeSnapsWithinOneMetre(t *testing.T) {
	// Aircraft is within 1 m of target altitude: should snap.
	s := cessnaAt(0, 0, 999.8)
	cmd := GotoPoint{Lat: 10.0, Lon: 0.0, Alt: 1000.0}

	got := Advance(s, cmd, noWind(), 1.0)

	if got.Alt != 1000.0 {
		t.Errorf("Goto alt snap: expected 1000.0 m, got %.3f", got.Alt)
	}
	if got.VAlt != 0 {
		t.Errorf("Goto alt snap: expected VAlt=0 after snap, got %.3f", got.VAlt)
	}
}

// ---- Hold -------------------------------------------------------------------

func TestAdvance_Hold_CountersWind(t *testing.T) {
	s := cessnaAt(5, 5, 100)
	wind := enabledWind(0.001, 0.002, 0)

	got := Advance(s, Hold{}, wind, 1.0)

	if got.VLat != -0.001 || got.VLon != -0.002 {
		t.Errorf("Hold: want VLat=-0.001 VLon=-0.002, got VLat=%.6f VLon=%.6f", got.VLat, got.VLon)
	}
}

func TestAdvance_Hold_VelocityIsConsistentOverMultipleTicks(t *testing.T) {
	// Hold applies constant anti-wind velocity every tick; the sign must stay
	// opposite to the wind across successive ticks.
	s := cessnaAt(5, 5, 100)
	wind := enabledWind(0.001, 0.0, 0)

	after1 := Advance(s, Hold{}, wind, 1.0)
	after2 := Advance(after1, Hold{}, wind, 1.0)

	if after2.VLat != -0.001 {
		t.Errorf("Hold multi-tick: VLat should stay -0.001, got %.6f", after2.VLat)
	}
}

// ---- SetHeading -------------------------------------------------------------

func TestAdvance_SetHeading_UpdatesHeading(t *testing.T) {
	s := cessnaAt(0, 0, 500)
	s.Heading = 0

	got := Advance(s, SetHeading{Heading: 90.0}, noWind(), 1.0)

	if got.Heading != 90.0 {
		t.Errorf("SetHeading: want 90.0°, got %.1f°", got.Heading)
	}
}

// ---- Trajectory -------------------------------------------------------------

func TestAdvance_Trajectory_AdvancesWaypointOnArrival(t *testing.T) {
	// Two waypoints; after enough ticks the first should be consumed.
	s := cessnaAt(0, 0, 1000)
	traj := &Trajectory{
		Waypoints: []Waypoint{
			{Lat: 0.0, Lon: 0.0, Alt: 1000.0}, // at origin — immediate arrival
			{Lat: 1.0, Lon: 0.0, Alt: 1000.0},
		},
	}

	Advance(s, traj, noWind(), 1.0)

	// The first waypoint (at origin) should have been consumed.
	if len(traj.Waypoints) != 1 {
		t.Errorf("Trajectory: expected 1 waypoint remaining, got %d", len(traj.Waypoints))
	}
}

// ---- bearingDeg -------------------------------------------------------------

func TestBearingDeg(t *testing.T) {
	tests := []struct {
		name         string
		lat1, lon1   float64
		lat2, lon2   float64
		wantDeg      float64
		toleranceDeg float64
	}{
		{"due north", 0, 0, 1, 0, 0.0, 1.0},
		{"due east", 0, 0, 0, 1, 90.0, 1.0},
		{"due south", 1, 0, 0, 0, 180.0, 1.0},
		{"due west", 0, 1, 0, 0, 270.0, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bearingDeg(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if math.Abs(got-tt.wantDeg) > tt.toleranceDeg {
				t.Errorf("bearingDeg(%s): got %.2f°, want ~%.2f°", tt.name, got, tt.wantDeg)
			}
		})
	}
}

// ---- clamp ------------------------------------------------------------------

func TestClamp(t *testing.T) {
	tests := []struct {
		v, lo, hi, want float64
	}{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{11, 0, 10, 10},
		{0, 0, 10, 0},
		{10, 0, 10, 10},
		{-5, -10, -1, -5},
	}
	for _, tt := range tests {
		if got := clamp(tt.v, tt.lo, tt.hi); got != tt.want {
			t.Errorf("clamp(%.1f, %.1f, %.1f) = %.1f, want %.1f", tt.v, tt.lo, tt.hi, got, tt.want)
		}
	}
}
