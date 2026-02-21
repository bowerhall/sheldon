# Sheldon Makefile

.PHONY: build build-sheldon build-coder push run stop logs shell status clean test

# Docker image names
SHELDON_IMAGE ?= ghcr.io/bowerhall/sheldon:latest
CODER_IMAGE ?= ghcr.io/bowerhall/sheldon-coder-sandbox:latest

# Build all container images
build: build-sheldon build-coder

# Build main sheldon image
build-sheldon:
	@echo "Building sheldon image..."
	docker build -t $(SHELDON_IMAGE) -f core/Dockerfile .

# Build coder sandbox image
build-coder:
	@echo "Building coder-sandbox image..."
	docker build -t $(CODER_IMAGE) deploy/docker/coder-sandbox/

# Push images to registry
push: push-sheldon push-coder

push-sheldon:
	docker push $(SHELDON_IMAGE)

push-coder:
	docker push $(CODER_IMAGE)

# Start services with Docker Compose
run:
	cd deploy/docker/standard && docker compose up -d

# Stop services
stop:
	cd deploy/docker/standard && docker compose down

# Restart sheldon (after rebuild)
restart:
	cd deploy/docker/standard && docker compose restart sheldon

# View logs
logs:
	cd deploy/docker/standard && docker compose logs -f sheldon

# View all logs
logs-all:
	cd deploy/docker/standard && docker compose logs -f

# Get shell in sheldon container
shell:
	cd deploy/docker/standard && docker compose exec sheldon /bin/sh

# Check status
status:
	cd deploy/docker/standard && docker compose ps

# Clean up
clean:
	cd deploy/docker/standard && docker compose down -v

# Run Go tests
test:
	cd core && go test ./...

# Build and run (full cycle)
all: build run

# Local development without Docker
run-local:
	cd core && go run ./cmd/sheldon

# Pull latest images
pull:
	cd deploy/docker/standard && docker compose pull
