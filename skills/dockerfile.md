---
name: dockerfile
description: Dockerfile patterns for Go and Node applications
version: 1.0.0
metadata:
  openclaw:
    requires:
      bins:
        - docker
---

# Dockerfile Patterns

## Go Applications

- multi-stage build (builder + runtime)
- use alpine for small images
- run as non-root user

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o app .

FROM alpine:3.19
RUN adduser -D -u 1000 app
COPY --from=builder /build/app /usr/local/bin/
USER app
EXPOSE 8080
CMD ["app"]
```

## Node Applications

```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .

FROM node:20-alpine
RUN adduser -D -u 1000 app
WORKDIR /app
COPY --from=builder /app .
USER app
EXPOSE 3000
CMD ["node", "index.js"]
```
