# Airborne Flight Simulator Backend (Go/Rust)

## Goal

Build a concurrent backend service that simulates an aircraft flying and allows clients to command it using either:

- Go to point (lat/lon/alt), or
- A trajectory (a list of waypoints / segments)

There may be no UI. A UI/visualization is a bonus.

---

## What You Will Build

### 1. Aircraft Simulation Engine

A simulation that advances over time (ticks), maintaining an aircraft state.

#### Aircraft State (Minimum)

- **position**: latitude, longitude, altitude (or x/y/z)
- **velocity**: ground speed (or vx/vy/vz)
- **heading**: or direction vector
- **timestamp**

#### Simulation Behavior

- Runs continuously (e.g., 10–60Hz tick or simulated-time steps)
- Moves the aircraft along the active command:
  - **go-to-point**: fly toward target until within tolerance
  - **trajectory**: follow waypoints in order (with a simple control law)

#### Flight Model

The flight model can be abstract. We are not testing aerodynamics accuracy; we are testing:

- Correctness and clarity
- Concurrency + architecture
- Robustness and observability

---

### 2. Command & Control API (Backend)

Expose an API to:

- Submit a go-to-point command
- Submit a trajectory
- Query current aircraft state
- Stream aircraft state (bonus)

You can implement either:

- HTTP JSON REST, or
- gRPC

#### Required Endpoints (Example REST)

**POST /command/goto**
```json
Body: { "lat": ..., "lon": ..., "alt": ..., "speed": optional }
```

**POST /command/trajectory**
```json
Body: { "waypoints": [ {lat, lon, alt, speed?}, ... ], "loop": optional }
```

**GET /state**
- Returns current state.

**GET /health**

#### Bonus Endpoints

- **GET /stream** — SSE or WebSocket for live state updates
- **POST /command/stop** — Stop current command
- **POST /command/hold** — Hold current position

---

### 3. Concurrency Requirements (Must-Have)

The system must use concurrency, not just "one goroutine and a mutex."

We want to see a senior approach, e.g.:

- Simulation loop in its own goroutine/task
- Command ingestion via channel / queue
- State publication via channels/pubsub
- Clean shutdown via context/cancellation
- Race-free state ownership (actor model encouraged)

---

### 4. Bonus: Environment Effects

Add a simple environment model that affects motion. Pick any 1–3:

- **Airflow / Wind**: add wind vector to ground track
- **Humidity**: impacts max speed or climb rate (simple coefficient)
- **DTM / Terrain**: ground altitude map (synthetic is fine)
  - Enforce "do not descend below terrain + safety margin"
  - Optionally: warn when commanded path intersects terrain

These should be modular: easy to add/disable.

---

## Deliverables

1. **A Git repo with:**
   - build/run instructions (README.md)
   - how to send commands (curl examples)
   - assumptions + tradeoffs
   
   Or a zip file.

2. **Optional:** a short design note with architecture overview
