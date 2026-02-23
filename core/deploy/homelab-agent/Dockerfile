# Homelab Agent
# Lightweight agent for remote machine management
#
# Build: docker build -f deploy/homelab-agent/Dockerfile -t homelab-agent .
# Run: docker run -d -p 8080:8080 -v /var/run/docker.sock:/var/run/docker.sock homelab-agent

FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /homelab-agent ./cmd/homelab-agent

# Runtime image
FROM alpine:3.19

RUN apk add --no-cache docker-cli ca-certificates

COPY --from=builder /homelab-agent /homelab-agent

EXPOSE 8080

ENV PORT=8080

ENTRYPOINT ["/homelab-agent"]
