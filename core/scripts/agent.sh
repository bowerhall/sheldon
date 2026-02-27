#!/bin/bash
set -e

# Sheldon Homelab Agent (standalone)
# Usage: curl bowerhall.ai/sheldon/agent | sudo bash

IMAGE="${AGENT_IMAGE:-ghcr.io/bowerhall/sheldon-homelab-agent:latest}"

echo "Installing homelab-agent..."

# Install Docker if missing
if ! command -v docker &> /dev/null; then
    echo "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
fi

# Stop existing container
docker rm -f homelab-agent 2>/dev/null || true

# Run agent
docker run -d \
    --name homelab-agent \
    --restart unless-stopped \
    -p 8080:8080 \
    -v /var/run/docker.sock:/var/run/docker.sock \
    "$IMAGE"

echo ""
echo "Done! Agent running on port 8080"
echo "Test: curl http://localhost:8080/health"
