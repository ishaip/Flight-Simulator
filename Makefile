.PHONY: run build test lint clean docker-build docker-run docker-compose-up build-all

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

## lint: run static analysis (golangci-lint if available, else go vet)
lint:
	@if command -v golangci-lint > /dev/null 2>&1; then \
		cd $(BACKEND_DIR) && golangci-lint run ./...; \
	else \
		echo "golangci-lint not found, falling back to go vet (install: https://golangci-lint.run/usage/install/)"; \
		cd $(BACKEND_DIR) && go vet ./...; \
	fi

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

# ── Cross-Platform Builds ─────────────────────────────────────────────────────

## build-all: build binaries for Linux, macOS (Intel), macOS (ARM), and Windows
build-all: clean
	mkdir -p bin
	@echo "Building for Linux (amd64)..."
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../bin/flight-simulator-linux-amd64 ./cmd/simulator
	@echo "Building for macOS (amd64/Intel)..."
	cd $(BACKEND_DIR) && GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ../bin/flight-simulator-macos-amd64 ./cmd/simulator
	@echo "Building for macOS (arm64/Apple Silicon)..."
	cd $(BACKEND_DIR) && GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../bin/flight-simulator-macos-arm64 ./cmd/simulator
	@echo "Building for Windows (amd64)..."
	cd $(BACKEND_DIR) && GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ../bin/flight-simulator-windows-amd64.exe ./cmd/simulator
	@echo "✓ All binaries built successfully in ./bin/"

# ── Help ─────────────────────────────────────────────────────────────────────
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
