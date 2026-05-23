# Flight Simulator

A real-time browser-based flight simulator with a Go backend and a vanilla JS frontend.

> Live at `http://localhost:8080` after running locally.

---

## Project Structure

```
flight-simulator/
├── frontend/               # Static web UI (HTML + CSS + JS — no build step)
│   ├── index.html          # HTML skeleton
│   ├── css/
│   │   └── app.css         # All styles
│   └── js/
│       └── app.js          # All UI logic + API calls
│
├── backend/                # Go HTTP server + simulation engine
│   ├── cmd/simulator/
│   │   └── main.go         # Entry point
│   ├── internal/
│   │   ├── api/            # HTTP handlers, routing, SSE stream
│   │   ├── clock/          # Global tick bus (real-time & fast-forward modes)
│   │   ├── env/            # Wind model
│   │   ├── log/            # Per-session structured logger + broadcaster
│   │   ├── session/        # Per-user session management
│   │   └── sim/            # Physics engine, state, commands, actor
│   ├── outputs/            # Trace files written at runtime
│   └── go.mod
│
├── Makefile                # Developer tasks: run, build, test, lint, docker-*
├── Dockerfile              # Multi-stage production image
├── docker-compose.yml      # One-command local run
├── .env.example            # Documented environment variables
└── .gitignore
```

---

## Quick Start

### Option 1: Docker (recommended, no Go required)

**Prerequisites:** [Docker Desktop](https://www.docker.com/products/docker-desktop)

```bash
git clone https://github.com/ishaip/Flight-Simulator.git
cd Flight-Simulator
docker-compose up --build
```

Then open `http://localhost:8080` in your browser.

### Option 2: Run from source

**Prerequisites:** [Go 1.22+](https://go.dev/dl/)

```bash
git clone https://github.com/ishaip/Flight-Simulator.git
cd Flight-Simulator/backend
go run ./cmd/simulator
```

Then open `http://localhost:8080` in your browser.

---

## Configuration

All configuration is via environment variables. See [`.env.example`](.env.example) for the full list.

| Variable     | Default         | Description                                 |
|--------------|-----------------|---------------------------------------------|
| `ADDR`       | `:8080`         | Address the HTTP server listens on          |
| `STATIC_DIR` | `../frontend`   | Path to serve frontend static files from    |

---

## Deployment & Self-Hosting

### Docker Compose (easiest)

```bash
git clone https://github.com/ishaip/Flight-Simulator.git
cd Flight-Simulator
docker-compose up -d
```

Then visit `http://localhost:8080`.

### Manual Docker

```bash
git clone https://github.com/ishaip/Flight-Simulator.git
cd Flight-Simulator
docker build -t flight-simulator:latest .
docker run -p 8080:8080 flight-simulator:latest
```

---

## API Reference

The full endpoint list is available at runtime:

```
GET http://localhost:8080/help
```

Key endpoints:

| Method | Path                   | Description                              |
|--------|------------------------|------------------------------------------|
| GET    | `/`                    | Serve frontend                           |
| GET    | `/state`               | Current aircraft state (JSON)            |
| GET    | `/stream`              | Server-Sent Events (state + log stream)  |
| POST   | `/command/trajectory`  | Set waypoints                            |
| POST   | `/command/stop`        | Stop aircraft                            |
| POST   | `/sim/pause`           | Pause simulation clock                   |
| POST   | `/sim/resume`          | Resume simulation clock                  |
| POST   | `/sim/hz`              | Set tick rate (10–60 Hz)                 |
| POST   | `/sim/skip`            | Fast-forward simulated time              |
| POST   | `/wind`                | Set wind vector                          |
| GET    | `/health`              | Health check                             |

---

## Architecture

```
Browser (SSE + REST)
      │
      ▼
  HTTP Server  (net/http, port 8080)
      │
      ├── SessionManager  — one SimulationActor per browser session
      │       └── SimulationActor  — goroutine; processes commands, runs physics
      │
      └── ClockBus  — global tick source (20 Hz default)
              ├── RealTime mode  — wall-clock paced
              └── FastForward mode  — CPU-speed, used for time-skip
```

Real-time updates flow via **Server-Sent Events**. State and log events are multiplexed on the
same `/stream` connection using the `event:` prefix.

---

## Development

```sh
make test    # go test ./...
make lint    # go vet ./...
```
