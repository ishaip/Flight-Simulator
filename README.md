# Flight Simulator

A real-time browser-based flight simulator with a Go backend and a vanilla JS frontend.

```
http://localhost:8080
```

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

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)

### Run in development

```sh
make run
```

The server starts on `http://localhost:8080`. The backend serves `../frontend` relative to its
working directory — no separate frontend server needed.

### Build a binary

```sh
make build
# outputs: bin/flight-simulator
```

### Run with Docker

**Prerequisites:**
- [Docker Desktop](https://www.docker.com/products/docker-desktop) (includes docker & docker-compose)

**On Linux/macOS:**
```sh
make docker-build
make docker-run
```

**On Windows (PowerShell):**
```powershell
# Skip 'make' — run docker directly
docker build -t flight-simulator:latest .
docker run --rm -p 8080:8080 flight-simulator:latest
```

**Or use Docker Compose (all platforms):**
```sh
docker-compose up --build
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

### Option 1: Docker (Recommended)

The easiest way to host the simulator. Pull the image from Docker Hub:

```bash
docker run -p 8080:8080 yourusername/flight-simulator:latest
```

Or build and run locally:

```bash
docker build -t flight-simulator:latest .
docker run -p 8080:8080 flight-simulator:latest
```

Then visit `http://localhost:8080` (or your server's IP).

### Option 2: Docker Compose

For production with environment configuration:

```bash
docker-compose up -d
```

Edit `docker-compose.yml` to customize the port and environment variables.

### Option 3: Native Binary (No Docker)

Download a pre-built binary from [GitHub Releases](../../releases):

**Linux (x64):**
```bash
wget https://github.com/yourusername/flight-simulator/releases/download/v1.0.0/flight-simulator-linux-amd64
chmod +x flight-simulator-linux-amd64
./flight-simulator-linux-amd64
```

**macOS (Intel):**
```bash
curl -L https://github.com/yourusername/flight-simulator/releases/download/v1.0.0/flight-simulator-macos-amd64 -o flight-simulator
chmod +x flight-simulator
./flight-simulator
```

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/yourusername/flight-simulator/releases/download/v1.0.0/flight-simulator-macos-arm64 -o flight-simulator
chmod +x flight-simulator
./flight-simulator
```

**Windows (PowerShell):**
```powershell
Invoke-WebRequest -Uri "https://github.com/yourusername/flight-simulator/releases/download/v1.0.0/flight-simulator-windows-amd64.exe" -OutFile "flight-simulator.exe"
.\flight-simulator.exe
```

Then open `http://localhost:8080` in your browser.

### Building Binaries for Multiple Platforms

To build binaries for all platforms locally:

```bash
make build-all
# outputs: bin/flight-simulator-linux-amd64, bin/flight-simulator-macos-amd64, bin/flight-simulator-macos-arm64, bin/flight-simulator-windows-amd64.exe
```

### CI/CD & Automated Releases

GitHub Actions automatically:
- Runs tests on every push and pull request
- Builds Docker images and publishes to Docker Hub (on tag push)
- Creates pre-built binaries for Windows, macOS (Intel & ARM), and Linux (on tag push)

To use this:

1. **Set up Docker Hub secrets** in GitHub (Settings → Secrets):
   - `DOCKER_USERNAME`: your Docker Hub username
   - `DOCKER_PASSWORD`: your Docker Hub token

2. **Tag a release:**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. **Binaries & Docker image are automatically built and released!**

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
