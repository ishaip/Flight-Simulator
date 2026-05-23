package sim

import (
	"sync"
	"testing"
	"time"
)

func TestStateStore_SetGet_RoundTrip(t *testing.T) {
	var s StateStore
	want := AircraftState{
		Lat:       51.5074,
		Lon:       -0.1278,
		Alt:       1000.0,
		Heading:   270.0,
		PlaneType: PlaneCirrus,
		SimTime:   time.Now().UTC(),
		Seq:       42,
	}
	s.Set(want)
	got := s.Get()

	if got.Lat != want.Lat || got.Lon != want.Lon || got.Alt != want.Alt {
		t.Errorf("position round-trip: got (%.4f, %.4f, %.1f), want (%.4f, %.4f, %.1f)",
			got.Lat, got.Lon, got.Alt, want.Lat, want.Lon, want.Alt)
	}
	if got.Heading != want.Heading {
		t.Errorf("heading round-trip: got %.1f, want %.1f", got.Heading, want.Heading)
	}
	if got.PlaneType != want.PlaneType {
		t.Errorf("planeType round-trip: got %q, want %q", got.PlaneType, want.PlaneType)
	}
	if got.Seq != want.Seq {
		t.Errorf("seq round-trip: got %d, want %d", got.Seq, want.Seq)
	}
}

func TestStateStore_GetReturnsACopy(t *testing.T) {
	var s StateStore
	s.Set(AircraftState{Lat: 1.0})

	got := s.Get()
	got.Lat = 99.0 // mutate the copy

	// The store must not reflect the mutation.
	if s.Get().Lat != 1.0 {
		t.Error("Get() returned a reference; mutations must not affect the store")
	}
}

// TestStateStore_ConcurrentAccess runs concurrent reads and writes and is
// designed to be caught by the race detector: go test -race ./internal/sim/...
func TestStateStore_ConcurrentAccess(t *testing.T) {
	var s StateStore
	s.Set(AircraftState{Lat: 0})

	const workers = 50
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for i := range workers {
		go func(i int) {
			defer wg.Done()
			s.Set(AircraftState{Lat: float64(i), Seq: uint64(i)})
		}(i)
		go func() {
			defer wg.Done()
			_ = s.Get()
		}()
	}
	wg.Wait()
}
