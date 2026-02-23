# Homelab Agent
# Lightweight agent for remote machine management
#
# Build: docker build -f deploy/homelab-agent/Dockerfile -t homelab-agent .
# Run: docker run -d -p 8080:8080 -v /var/run/docker.sock:/var/run/docker.sock homelab-agent

FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files from homelab-agent directory
COPY cmd/homelab-agent/go.mod cmd/homelab-agent/go.sum ./
RUN go mod download

# Copy source
COPY cmd/homelab-agent/*.go ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /homelab-agent .

# Runtime image
FROM alpine:3.19

RUN apk add --no-cache docker-cli ca-certificates

COPY --from=builder /homelab-agent /homelab-agent

EXPOSE 8080

ENV PORT=8080

ENTRYPOINT ["/homelab-agent"]
