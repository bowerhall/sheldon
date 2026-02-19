FROM golang:1.24-alpine AS builder

WORKDIR /build

COPY core/go.mod core/go.sum ./core/
COPY pkg/koramem/go.mod pkg/koramem/go.sum ./pkg/koramem/

WORKDIR /build/core
RUN go mod download

WORKDIR /build
COPY core/ ./core/
COPY pkg/koramem/ ./pkg/koramem/

WORKDIR /build/core
RUN CGO_ENABLED=0 GOOS=linux go build -o /kora ./cmd/kora

FROM alpine:3.19

RUN apk update && \
    apk add --no-cache ca-certificates tzdata nodejs npm docker-cli kubectl && \
    npm install -g @anthropic-ai/claude-code && \
    npm cache clean --force && \
    adduser -D -u 1000 kora && \
    addgroup kora docker 2>/dev/null || true && \
    mkdir -p /data /tmp/kora-sandbox && \
    chown -R kora:kora /data /tmp/kora-sandbox

COPY --from=builder /kora /usr/local/bin/kora

USER kora
WORKDIR /home/kora

ENTRYPOINT ["/usr/local/bin/kora"]
