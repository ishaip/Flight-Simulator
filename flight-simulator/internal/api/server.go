package api

import (
	"context"
	"log"
	"net/http"

	"flight-simulator/internal/clock"
	"flight-simulator/internal/session"
)

// Server is the HTTP server.
type Server struct {
	addr string
	srv  *http.Server
}

// NewServer wires all routes and returns a Server ready to listen.
func NewServer(
	addr string,
	manager *session.Manager,
	bus *clock.ClockBus,
	staticFS http.Handler,
) *Server {
	d := &deps{manager: manager, bus: bus}

	mux := http.NewServeMux()

	// Simulation commands.
	mux.HandleFunc("POST /command/goto", d.handleGoto)
	mux.HandleFunc("POST /command/trajectory", d.handleTrajectory)
	mux.HandleFunc("POST /command/stop", d.handleStop)
	mux.HandleFunc("POST /command/hold", d.handleHold)
	mux.HandleFunc("POST /command/accelerate", d.handleAccelerate)
	mux.HandleFunc("POST /command/trace/start", d.handleTraceStart)

	// State & health.
	mux.HandleFunc("GET /state", d.handleState)
	mux.HandleFunc("GET /stream", d.handleStream)
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /help", handleHelp)

	// Sim control.
	mux.HandleFunc("POST /sim/pause", d.handlePause)
	mux.HandleFunc("POST /sim/resume", d.handleResume)
	mux.HandleFunc("POST /sim/hz", d.handleHz)
	mux.HandleFunc("POST /sim/skip", d.handleSkip)

	// Wind.
	mux.HandleFunc("POST /wind", d.handleWind)

	// Frontend — serve web/index.html at /.
	mux.Handle("/", staticFS)

	return &Server{
		addr: addr,
		srv:  &http.Server{Addr: addr, Handler: mux},
	}
}

// ListenAndServe starts the HTTP server. It returns when the server stops.
func (s *Server) ListenAndServe() error {
	log.Printf("flight-simulator listening on http://localhost%s", s.addr)
	return s.srv.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
