package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"flight-simulator/internal/sim"
)

// streamHandler pushes the aircraft state as Server-Sent Events to a connected client.
// Each SSE message is a JSON-encoded AircraftState prefixed with "data: ".
//
// The client can connect with:
//
//	new EventSource('/stream?sid=<sessionID>')
func (d *deps) handleStream(w http.ResponseWriter, r *http.Request) {
	// Get or create session
	sessionID := getSessionID(r)
	planeType := sim.PlaneType(getPlaneType(r))
	sess := d.manager.GetOrCreate(sessionID, planeType)
	setSessionIDCookie(w, sess.ID)

	// Verify the client supports SSE.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := sess.Broadcaster.Subscribe()
	defer sess.Broadcaster.Unsubscribe(ch)

	logCh := sess.LogBroadcaster.Subscribe()
	defer sess.LogBroadcaster.Unsubscribe(logCh)

	sess.Logger.Trace("SSE client connected")

	for {
		select {
		case state, open := <-ch:
			if !open {
				return
			}
			data, err := json.Marshal(state)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case logEntry, open := <-logCh:
			if !open {
				return
			}
			logData, err := json.Marshal(logEntry)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: log\ndata: %s\n\n", logData)
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}
