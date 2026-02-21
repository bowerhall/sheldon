#!/bin/bash
# Sheldon Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/bowerhall/sheldon/main/core/deploy/install.sh | bash
set -e

SHELDON_DIR="${SHELDON_DIR:-/opt/sheldon}"
REPO_URL="https://raw.githubusercontent.com/bowerhall/sheldon/main/core"

echo "================================================"
echo "  Sheldon Installer"
echo "================================================"
echo ""

# Check for Docker
if ! command -v docker &> /dev/null; then
    echo "Docker not found. Installing..."
    curl -fsSL https://get.docker.com | sh
    sudo usermod -aG docker "$USER"
    echo ""
    echo "Docker installed. You may need to log out and back in for group changes."
    echo "Then re-run this script."
    exit 0
fi

echo "Docker found: $(docker --version)"
echo ""

echo "Setting up Sheldon..."
mkdir -p "$SHELDON_DIR"
cd "$SHELDON_DIR"

# Download compose files
curl -fsSL "$REPO_URL/deploy/docker-compose.yml" -o docker-compose.yml
curl -fsSL "$REPO_URL/deploy/apps.yml" -o apps.yml
curl -fsSL "$REPO_URL/.env.example" -o .env.example

# Create directories
mkdir -p data skills letsencrypt

# Copy env example if .env doesn't exist
if [ ! -f .env ]; then
    cp .env.example .env
fi

# Create network if it doesn't exist
docker network create sheldon-net 2>/dev/null || true

echo ""
echo "================================================"
echo "  Setup complete!"
echo "================================================"
echo ""
echo "Next steps:"
echo ""
echo "  1. Edit the configuration:"
echo "     nano $SHELDON_DIR/.env"
echo ""
echo "  2. Start Sheldon:"
echo "     cd $SHELDON_DIR"
echo "     docker compose up -d"
echo ""
echo "  3. View logs:"
echo "     docker compose logs -f sheldon"
echo ""
echo "  4. Message your Telegram bot!"
echo ""
