package api

import (
	"encoding/json"
	"net/http"
	"time"

	"flight-simulator/internal/clock"
	"flight-simulator/internal/session"
	"flight-simulator/internal/sim"
)

// deps bundles all handler dependencies.
type deps struct {
	manager *session.Manager
	bus     *clock.ClockBus
}

// getSessionID extracts the session ID from the request (query parameter or cookie).
func getSessionID(r *http.Request) string {
	if id := r.URL.Query().Get("sid"); id != "" {
		return id
	}
	if cookie, err := r.Cookie("sid"); err == nil {
		return cookie.Value
	}
	return ""
}

// setSessionIDCookie sets the session ID as a cookie and query parameter hint.
func setSessionIDCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "sid",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400 * 365, // 1 year
	})
}

// writeJSON writes v as a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// decodeBody decodes the JSON request body into v and returns false on error.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

// ---- /command/goto ----

func (d *deps) handleGoto(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	sess := d.manager.GetOrCreate(sessionID)
	setSessionIDCookie(w, sess.ID)

	var body struct {
		Lat   float64 `json:"lat"`
		Lon   float64 `json:"lon"`
		Alt   float64 `json:"alt"`
		Speed float64 `json:"speed"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	sess.Actor.SendCommand(sim.GotoPoint{Lat: body.Lat, Lon: body.Lon, Alt: body.Alt, Speed: body.Speed})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// ---- /command/trajectory ----

func (d *deps) handleTrajectory(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	sess := d.manager.GetOrCreate(sessionID)
	setSessionIDCookie(w, sess.ID)

	var body struct {
		Waypoints []sim.Waypoint `json:"waypoints"`
		Loop      bool           `json:"loop"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if len(body.Waypoints) == 0 {
		http.Error(w, "waypoints must not be empty", http.StatusBadRequest)
		return
	}
	sess.Actor.SendCommand(&sim.Trajectory{Waypoints: body.Waypoints, Loop: body.Loop})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// ---- /command/stop ----

func (d *deps) handleStop(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	sess := d.manager.GetOrCreate(sessionID)
	setSessionIDCookie(w, sess.ID)

	sess.Actor.SendCommand(sim.Reset{})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "simulator reset"})
}

// ---- /command/hold ----

func (d *deps) handleHold(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	sess := d.manager.GetOrCreate(sessionID)
	setSessionIDCookie(w, sess.ID)

	sess.Actor.SendCommand(sim.Hold{})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// ---- /command/accelerate ----

func (d *deps) handleAccelerate(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	sess := d.manager.GetOrCreate(sessionID)
	setSessionIDCookie(w, sess.ID)

	var body struct {
		Value float64 `json:"value"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	sess.Actor.SendCommand(sim.Accelerate{Value: body.Value})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// ---- /command/trace/start ----

func (d *deps) handleTraceStart(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	sess := d.manager.GetOrCreate(sessionID)
	setSessionIDCookie(w, sess.ID)

	// Stub: full trace spec TBD.
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "trace started (stub)"})
}

// ---- /state ----

func (d *deps) handleState(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	sess := d.manager.GetOrCreate(sessionID)
	setSessionIDCookie(w, sess.ID)

	writeJSON(w, http.StatusOK, sess.Store.Get())
}

// ---- /health ----

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- /sim/pause ----

func (d *deps) handlePause(w http.ResponseWriter, r *http.Request) {
	d.bus.Pause()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "paused"})
}

// ---- /sim/resume ----

func (d *deps) handleResume(w http.ResponseWriter, r *http.Request) {
	d.bus.Resume()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "resumed"})
}

// ---- /sim/hz ----

func (d *deps) handleHz(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Hz float64 `json:"hz"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	d.bus.SetHz(body.Hz)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "hz updated"})
}

// ---- /sim/skip ----

func (d *deps) handleSkip(w http.ResponseWriter, r *http.Request) {
	var body struct {
		By string `json:"by"` // e.g. "30s", "5m"
		To string `json:"to"` // ISO 8601
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.By != "" {
		dur, err := time.ParseDuration(body.By)
		if err != nil {
			http.Error(w, "invalid duration: "+err.Error(), http.StatusBadRequest)
			return
		}
		d.bus.SkipBy(dur)
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "fast-forwarding"})
		return
	}
	if body.To != "" {
		t, err := time.Parse(time.RFC3339, body.To)
		if err != nil {
			http.Error(w, "invalid time (RFC3339): "+err.Error(), http.StatusBadRequest)
			return
		}
		d.bus.SkipTo(t)
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "fast-forwarding"})
		return
	}
	http.Error(w, `supply "by" or "to"`, http.StatusBadRequest)
}

// ---- /wind ----

func (d *deps) handleWind(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	sess := d.manager.GetOrCreate(sessionID)
	setSessionIDCookie(w, sess.ID)

	var body struct {
		Enabled *bool   `json:"enabled"`
		VLat    float64 `json:"vLat"`
		VLon    float64 `json:"vLon"`
		VAlt    float64 `json:"vAlt"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.Enabled != nil {
		sess.Wind.SetEnabled(*body.Enabled)
	}
	sess.Wind.SetVector(body.VLat, body.VLon, body.VAlt)
	writeJSON(w, http.StatusAccepted, sess.Wind.Get())
}

// ---- /help ----

var helpDoc = map[string]any{
	"endpoints": []map[string]any{
		{"method": "POST", "path": "/command/goto", "body": map[string]string{"lat": "float", "lon": "float", "alt": "float (m)", "speed": "float m/s (optional)"}, "description": "Fly to a geographic point"},
		{"method": "POST", "path": "/command/trajectory", "body": map[string]string{"waypoints": "[{lat,lon,alt,speed?}]", "loop": "bool (optional)"}, "description": "Follow a sequence of waypoints"},
		{"method": "POST", "path": "/command/stop", "description": "Stop movement immediately"},
		{"method": "POST", "path": "/command/hold", "description": "Station-keep at current position (counters wind)"},
		{"method": "POST", "path": "/command/accelerate", "body": map[string]string{"value": "float m/s² (negative = decelerate)"}, "description": "Apply throttle along current heading"},
		{"method": "POST", "path": "/command/trace/start", "description": "Start flight-path trace (stub)"},
		{"method": "GET", "path": "/state", "description": "Current aircraft state snapshot"},
		{"method": "GET", "path": "/stream", "description": "SSE stream of aircraft state (text/event-stream)"},
		{"method": "GET", "path": "/health", "description": "Liveness check"},
		{"method": "POST", "path": "/sim/pause", "description": "Pause the simulation clock"},
		{"method": "POST", "path": "/sim/resume", "description": "Resume the simulation clock"},
		{"method": "POST", "path": "/sim/hz", "body": map[string]string{"hz": "float 10–60"}, "description": "Set tick rate (applied after current tick)"},
		{"method": "POST", "path": "/sim/skip", "body": map[string]string{"by": "duration string e.g. 30s, 5m", "to": "RFC3339 timestamp"}, "description": "Fast-forward simulated time"},
		{"method": "POST", "path": "/wind", "body": map[string]string{"enabled": "bool", "vLat": "float °/s", "vLon": "float °/s", "vAlt": "float m/s"}, "description": "Configure wind model"},
		{"method": "GET", "path": "/help", "description": "This document"},
	},
}

func handleHelp(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, helpDoc)
}
