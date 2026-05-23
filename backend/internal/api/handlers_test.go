package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleHealth verifies the health endpoint returns OK status.
func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}
}

// TestHandleShutdown verifies the shutdown endpoint returns OK status before exit.
func TestHandleShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping shutdown test in short mode (exits process)")
	}

	req := httptest.NewRequest("POST", "/shutdown", nil)
	w := httptest.NewRecorder()

	// We can't fully test os.Exit() in a test without killing the process,
	// so we only verify the HTTP response is sent correctly before shutdown is initiated.
	handleShutdown(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}
}
