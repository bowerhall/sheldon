FROM golang:1.22-alpine AS builder

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

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /kora /usr/local/bin/kora

ENTRYPOINT ["/usr/local/bin/kora"]
