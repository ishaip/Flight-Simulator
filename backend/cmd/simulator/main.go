package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"flight-simulator/internal/api"
	"flight-simulator/internal/clock"
	"flight-simulator/internal/session"
)

func main() {
	// ── Shared shutdown context ──────────────────────────────────────────────
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Global clock bus (shared by all sessions, 20 Hz default) ──────────────
	bus := clock.New(time.Now().UTC())

	// ── Session manager (creates per-user simulation instances) ──────────────
	manager := session.New(bus)

	// ── HTTP server ───────────────────────────────────────────────────────────
	// STATIC_DIR defaults to "../frontend" (relative to backend/ working dir).
	// Override via env var for Docker or alternative layouts.
	staticDir := "../frontend"
	if d := os.Getenv("STATIC_DIR"); d != "" {
		staticDir = d
	}
	staticFS := http.FileServer(http.Dir(staticDir))

	addr := ":8080"
	if a := os.Getenv("ADDR"); a != "" {
		addr = a
	}
	srv := api.NewServer(addr, manager, bus, staticFS)

	// ── Start goroutines ──────────────────────────────────────────────────────
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		bus.Run(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http server error: %v", err)
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	<-ctx.Done()
	log.Println("shutting down…")

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}

	wg.Wait()
	log.Println("stopped")
}
