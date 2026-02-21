#!/bin/bash
# Local testing script for Sheldon
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORE_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== Sheldon Local Test ==="
echo ""

# Check for .env file
if [ ! -f "$SCRIPT_DIR/standard/.env" ]; then
    echo "Error: No .env file found"
    echo ""
    echo "Setup:"
    echo "  cd $SCRIPT_DIR/standard"
    echo "  cp .env.example .env"
    echo "  # Edit .env with your tokens"
    echo "  cd .. && ./test-local.sh"
    exit 1
fi

echo "1. Building Sheldon image..."
docker build -t sheldon:local -f "$CORE_DIR/Dockerfile" "$CORE_DIR/.."

echo ""
echo "2. Building coder-sandbox image..."
"$SCRIPT_DIR/docker/coder-sandbox/build.sh"

echo ""
echo "3. Creating network..."
docker network create sheldon-net 2>/dev/null || true

echo ""
echo "4. Starting services..."
cd "$SCRIPT_DIR/standard"

# Override image to use local build
SHELDON_IMAGE=sheldon:local docker compose up -d

echo ""
echo "=== Sheldon is running! ==="
echo ""
echo "Logs:     docker compose logs -f"
echo "Stop:     docker compose down"
echo "Traefik:  http://localhost:8080"
