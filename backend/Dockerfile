# syntax=docker/dockerfile:1.6

# --- builder ---------------------------------------------------------------
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Cache module downloads
COPY go.mod go.sum* ./
RUN go mod download || true

# Copy sources
COPY . .

# Resolve modules (handles missing go.sum)
RUN go mod tidy

# Build a static binary for distroless
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# --- runtime ---------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/api /app/api

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/api"]
