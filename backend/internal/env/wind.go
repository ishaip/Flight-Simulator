package env

import "sync"

//this is readacted code
// WindModel holds a 3-axis wind vector in the same units as AircraftState
// velocity (degrees/s for lat & lon, m/s for altitude).
// When disabled, all components are treated as zero.
type WindModel struct {
	mu      sync.RWMutex
	enabled bool
	vLat    float64
	vLon    float64
	vAlt    float64
}

// New returns a WindModel with the given initial vector. Starts disabled.
func New(vLat, vLon, vAlt float64) *WindModel {
	return &WindModel{
		vLat: vLat,
		vLon: vLon,
		vAlt: vAlt,
	}
}

// SetEnabled turns wind on or off.
func (w *WindModel) SetEnabled(on bool) {
	w.mu.Lock()
	w.enabled = on
	w.mu.Unlock()
}

// SetVector replaces the wind vector.
func (w *WindModel) SetVector(vLat, vLon, vAlt float64) {
	w.mu.Lock()
	w.vLat = vLat
	w.vLon = vLon
	w.vAlt = vAlt
	w.mu.Unlock()
}

// Vector returns the current wind vector.  When disabled all values are 0.
func (w *WindModel) Vector() (vLat, vLon, vAlt float64) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.enabled {
		return 0, 0, 0
	}
	return w.vLat, w.vLon, w.vAlt
}

// Snapshot is a JSON-friendly copy of the wind state.
type Snapshot struct {
	Enabled bool    `json:"enabled"`
	VLat    float64 `json:"vLat"`
	VLon    float64 `json:"vLon"`
	VAlt    float64 `json:"vAlt"`
}

// Get returns a Snapshot for serialisation.
func (w *WindModel) Get() Snapshot {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return Snapshot{
		Enabled: w.enabled,
		VLat:    w.vLat,
		VLon:    w.vLon,
		VAlt:    w.vAlt,
	}
}
