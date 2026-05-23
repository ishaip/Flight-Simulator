# syntax=docker/dockerfile:1

# ── Stage 1: Build the Go binary ─────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Cache dependencies first (go.mod has no external deps, but good practice)
COPY backend/go.mod ./
RUN go mod download

# Copy source and build
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /flight-simulator ./cmd/simulator

# ── Stage 2: Minimal runtime image ───────────────────────────────────────────
FROM alpine:3.19

# Non-root user for security
RUN addgroup -S app && adduser -S app -G app
WORKDIR /app

# Copy binary and frontend assets
COPY --from=builder /flight-simulator ./flight-simulator
COPY frontend/ ./frontend/

# Run as non-root
USER app

# STATIC_DIR is relative to the working directory /app
ENV STATIC_DIR=./frontend
ENV ADDR=:8080

EXPOSE 8080
CMD ["./flight-simulator"]
