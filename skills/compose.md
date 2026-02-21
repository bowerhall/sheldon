---
name: compose
description: Docker Compose patterns with Traefik routing
version: 1.0.0
metadata:
  openclaw:
    requires:
      bins:
        - docker
---

# Docker Compose Patterns

## Service Definition

- use specific image tags, not :latest
- set restart policy (unless-stopped)
- define healthcheck for critical services
- use named volumes for persistent data

## Traefik Routing

- add labels for automatic discovery
- use Host rules for subdomain routing

## Example

```yaml
services:
  myapp:
    image: myapp:1.0.0
    restart: unless-stopped
    networks:
      - sheldon-net
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.myapp.rule=Host(`myapp.${DOMAIN}`)"
      - "traefik.http.routers.myapp.entrypoints=web"
      - "traefik.http.services.myapp.loadbalancer.server.port=8080"

networks:
  sheldon-net:
    external: true
```

## Multi-Service App

```yaml
services:
  web:
    build: ./web
    depends_on:
      - api
    labels:
      - "traefik.http.routers.web.rule=Host(`app.${DOMAIN}`)"

  api:
    build: ./api
    depends_on:
      - db

  db:
    image: postgres:16-alpine
    volumes:
      - db-data:/var/lib/postgresql/data
    environment:
      - POSTGRES_PASSWORD=${DB_PASSWORD}
```
