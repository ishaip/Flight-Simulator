package api

import (
	"encoding/json"
	"fmt"
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

// getPlaneType extracts the plane type from the request (query parameter).
func getPlaneType(r *http.Request) string {
	return r.URL.Query().Get("planeType")
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
// The body is limited to 64 KB to prevent resource exhaustion.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16) // 64 KB
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

// ---- /command/goto ----

func (d *deps) handleGoto(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
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
	if body.Lat < -90 || body.Lat > 90 {
		http.Error(w, "lat must be in [-90, 90]", http.StatusBadRequest)
		return
	}
	if body.Lon < -180 || body.Lon > 180 {
		http.Error(w, "lon must be in [-180, 180]", http.StatusBadRequest)
		return
	}
	if body.Alt < 0 {
		http.Error(w, "alt must be >= 0", http.StatusBadRequest)
		return
	}
	if body.Speed < 0 {
		http.Error(w, "speed must be >= 0", http.StatusBadRequest)
		return
	}
	sess.Logger.Trace(fmt.Sprintf("command/goto: lat=%.6f lon=%.6f alt=%.1f speed=%.1f", body.Lat, body.Lon, body.Alt, body.Speed))
	sess.Logger.Info(fmt.Sprintf("User command: goto to (%.6f, %.6f) at %.1f m altitude with %.1f m/s", body.Lat, body.Lon, body.Alt, body.Speed))
	sess.Actor.SendCommand(sim.GotoPoint{Lat: body.Lat, Lon: body.Lon, Alt: body.Alt, Speed: body.Speed})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// ---- /command/trajectory ----

func (d *deps) handleTrajectory(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
	setSessionIDCookie(w, sess.ID)

	var body struct {
		Waypoints []sim.Waypoint `json:"waypoints"`
		Loop      bool           `json:"loop"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	const maxWaypoints = 500
	if len(body.Waypoints) == 0 {
		http.Error(w, "waypoints must not be empty", http.StatusBadRequest)
		return
	}
	if len(body.Waypoints) > maxWaypoints {
		http.Error(w, fmt.Sprintf("waypoints must not exceed %d", maxWaypoints), http.StatusBadRequest)
		return
	}
	for i, wp := range body.Waypoints {
		if wp.Lat < -90 || wp.Lat > 90 {
			http.Error(w, fmt.Sprintf("waypoint %d: lat must be in [-90, 90]", i), http.StatusBadRequest)
			return
		}
		if wp.Lon < -180 || wp.Lon > 180 {
			http.Error(w, fmt.Sprintf("waypoint %d: lon must be in [-180, 180]", i), http.StatusBadRequest)
			return
		}
		if wp.Alt < 0 {
			http.Error(w, fmt.Sprintf("waypoint %d: alt must be >= 0", i), http.StatusBadRequest)
			return
		}
		if wp.Speed < 0 {
			http.Error(w, fmt.Sprintf("waypoint %d: speed must be >= 0", i), http.StatusBadRequest)
			return
		}
	}
	sess.Actor.SendCommand(&sim.Trajectory{Waypoints: body.Waypoints, Loop: body.Loop})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// ---- /command/stop ----

func (d *deps) handleStop(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
	setSessionIDCookie(w, sess.ID)

	sess.Logger.Trace("command/stop")
	sess.Logger.Info("User command: stop")
	sess.Actor.SendCommand(sim.Stop{})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "stopped"})
}

// ---- /command/set-heading ----

func (d *deps) handleSetHeading(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
	setSessionIDCookie(w, sess.ID)

	var body struct {
		Heading float64 `json:"heading"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.Heading < 0 || body.Heading >= 360 {
		http.Error(w, "heading must be in [0, 360)", http.StatusBadRequest)
		return
	}
	sess.Logger.Trace(fmt.Sprintf("command/set-heading: heading=%.1f", body.Heading))
	sess.Logger.Info(fmt.Sprintf("User command: set heading to %.1f degrees", body.Heading))
	sess.Actor.SendCommand(sim.SetHeading{Heading: body.Heading})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// ---- /command/hold ----

func (d *deps) handleHold(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
	setSessionIDCookie(w, sess.ID)

	sess.Actor.SendCommand(sim.Hold{})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// ---- /command/trace/start ----

func (d *deps) handleTraceStart(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
	setSessionIDCookie(w, sess.ID)

	// Stub: full trace spec TBD.
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "trace started (stub)"})
}

// ---- /state ----

func (d *deps) handleState(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
	setSessionIDCookie(w, sess.ID)

	writeJSON(w, http.StatusOK, sess.Store.Get())
}

// ---- /health ----

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- /sim/pause ----

func (d *deps) handlePause(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
	sess.Logger.Trace("sim/pause")
	sess.Logger.Info("User command: pause simulation")
	d.bus.Pause()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "paused"})
}

// ---- /sim/resume ----

func (d *deps) handleResume(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
	sess.Logger.Trace("sim/resume")
	sess.Logger.Info("User command: resume simulation")
	d.bus.Resume()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "resumed"})
}

// ---- /sim/hz ----

func (d *deps) handleHz(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)

	var body struct {
		Hz float64 `json:"hz"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.Hz < clock.MinHz || body.Hz > clock.MaxHz {
		http.Error(w, fmt.Sprintf("hz must be in [%.0f, %.0f]", clock.MinHz, clock.MaxHz), http.StatusBadRequest)
		return
	}
	sess.Logger.Trace(fmt.Sprintf("sim/hz: hz=%.1f", body.Hz))
	sess.Logger.Info(fmt.Sprintf("User setting: simulator speed changed to %.1f Hz", body.Hz))
	d.bus.SetHz(body.Hz)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "hz updated"})
}

// ---- /sim/skip ----

func (d *deps) handleSkip(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)

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
		sess.Logger.Trace(fmt.Sprintf("sim/skip: by=%s", body.By))
		sess.Logger.Info(fmt.Sprintf("User command: skip time by %s", body.By))
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
		sess.Logger.Trace(fmt.Sprintf("sim/skip: to=%s", body.To))
		sess.Logger.Info(fmt.Sprintf("User command: skip time to %s", body.To))
		d.bus.SkipTo(t)
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "fast-forwarding"})
		return
	}
	http.Error(w, `supply "by" or "to"`, http.StatusBadRequest)
}

// ---- /wind ----

func (d *deps) handleWind(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
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
		sess.Logger.Trace(fmt.Sprintf("wind: enabled=%v", *body.Enabled))
		sess.Logger.Info(fmt.Sprintf("User setting: wind %s", map[bool]string{true: "enabled", false: "disabled"}[*body.Enabled]))
		sess.Wind.SetEnabled(*body.Enabled)
	}
	sess.Logger.Trace(fmt.Sprintf("wind: vLat=%.6f vLon=%.6f vAlt=%.2f", body.VLat, body.VLon, body.VAlt))
	sess.Wind.SetVector(body.VLat, body.VLon, body.VAlt)
	writeJSON(w, http.StatusAccepted, sess.Wind.Get())
}

// ---- /log/info ----

func (d *deps) handleLogInfo(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
	setSessionIDCookie(w, sess.ID)

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	sess.Logger.Trace(fmt.Sprintf("log/info: enabled=%v", body.Enabled))
	sess.Logger.SetInfoEnabled(body.Enabled)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "info logging updated"})
}

// ---- /log/trace ----

func (d *deps) handleLogTrace(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
	setSessionIDCookie(w, sess.ID)

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	sess.Logger.Trace(fmt.Sprintf("log/trace: enabled=%v", body.Enabled))
	sess.Logger.SetTraceEnabled(body.Enabled)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "trace logging updated"})
}

// ---- /help ----

var helpDoc = map[string]any{
	"endpoints": []map[string]any{
		{"method": "POST", "path": "/command/goto", "body": map[string]string{"lat": "float", "lon": "float", "alt": "float (m)", "speed": "float m/s (optional)"}, "description": "Fly to a geographic point"},
		{"method": "POST", "path": "/command/trajectory", "body": map[string]string{"waypoints": "[{lat,lon,alt,speed?}]", "loop": "bool (optional)"}, "description": "Follow a sequence of waypoints"},
		{"method": "POST", "path": "/command/set-heading", "body": map[string]string{"heading": "float degrees"}, "description": "Set heading without changing speed"},
		{"method": "POST", "path": "/command/stop", "description": "Stop movement immediately"},
		{"method": "POST", "path": "/command/hold", "description": "Station-keep at current position (counters wind)"},
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
