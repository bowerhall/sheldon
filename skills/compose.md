# docker compose patterns

## service definition
- use specific image tags, not :latest in production
- set restart policy (unless-stopped recommended)
- define healthcheck for critical services
- use named volumes for persistent data

## traefik routing
- add labels for automatic discovery
- use entrypoints (web, websecure)
- set Host rules for subdomain routing

## example
```yaml
# docker-compose.yml
services:
  {{name}}:
    image: {{image}}:{{tag}}
    restart: unless-stopped
    networks:
      - sheldon-net
    volumes:
      - {{name}}-data:/data
    environment:
      - NODE_ENV=production
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.{{name}}.rule=Host(`{{name}}.${DOMAIN}`)"
      - "traefik.http.routers.{{name}}.entrypoints=web"
      - "traefik.http.services.{{name}}.loadbalancer.server.port=8080"

volumes:
  {{name}}-data:

networks:
  sheldon-net:
    external: true
```

## multi-service app
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
    labels:
      - "traefik.http.routers.api.rule=Host(`api.${DOMAIN}`)"

  db:
    image: postgres:16-alpine
    volumes:
      - db-data:/var/lib/postgresql/data
    environment:
      - POSTGRES_PASSWORD=${DB_PASSWORD}
```
