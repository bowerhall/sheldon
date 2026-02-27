FROM golang:1.24-alpine AS builder

WORKDIR /build

COPY core/go.mod core/go.sum ./core/
COPY pkg/sheldonmem/go.mod pkg/sheldonmem/go.sum ./pkg/sheldonmem/

WORKDIR /build/core
RUN go mod download

WORKDIR /build
COPY core/ ./core/
COPY pkg/sheldonmem/ ./pkg/sheldonmem/

WORKDIR /build/core
RUN CGO_ENABLED=0 GOOS=linux go build -o /sheldon ./cmd/sheldon

FROM alpine:3.19

# System dependencies (rarely changes - cached)
RUN apk add --no-cache ca-certificates tzdata nodejs npm docker-cli docker-cli-compose

# npm packages (separate layer for better caching)
RUN npm install -g @anthropic-ai/claude-code && npm cache clean --force

# User setup
RUN adduser -D -u 1000 sheldon && \
    addgroup sheldon docker 2>/dev/null || true && \
    mkdir -p /data /tmp/sheldon-sandbox && \
    chown -R sheldon:sheldon /data /tmp/sheldon-sandbox

COPY --from=builder /sheldon /usr/local/bin/sheldon
COPY --from=builder /build/core/essence /home/sheldon/essence

USER sheldon
WORKDIR /home/sheldon

ENTRYPOINT ["/usr/local/bin/sheldon"]
