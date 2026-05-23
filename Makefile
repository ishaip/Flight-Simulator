.PHONY: run build test lint clean docker-build docker-run

BACKEND_DIR  := backend
FRONTEND_DIR := frontend
BINARY       := bin/flight-simulator

# ── Development ──────────────────────────────────────────────────────────────

## run: start the server in development mode (hot path: backend/ serves ../frontend/)
run:
	cd $(BACKEND_DIR) && go run ./cmd/simulator

## build: compile to bin/flight-simulator
build:
	mkdir -p bin
	cd $(BACKEND_DIR) && go build -o ../$(BINARY) ./cmd/simulator

## test: run all Go tests
test:
	cd $(BACKEND_DIR) && go test ./... -v

## lint: run static analysis
lint:
	cd $(BACKEND_DIR) && go vet ./...

## clean: remove build outputs
clean:
	rm -rf bin/
	rm -f $(BACKEND_DIR)/outputs/*.json $(BACKEND_DIR)/outputs/*.csv

# ── Docker ────────────────────────────────────────────────────────────────────

## docker-build: build the production Docker image
docker-build:
	docker build -t flight-simulator:latest .

## docker-run: run the image on port 8080
docker-run:
	docker run --rm -p 8080:8080 flight-simulator:latest

## docker-compose-up: start via docker-compose
docker-compose-up:
	docker-compose up --build

# ── Help ─────────────────────────────────────────────────────────────────────
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
