#!/bin/bash
# Sheldon Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/bowerhall/sheldon/main/deploy/install.sh | bash
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

# Choose setup mode
echo "How do you want to run Sheldon?"
echo ""
echo "  1) Minimal"
echo "     Just the chatbot in a single container."
echo "     No web routing, no app deployment."
echo "     Good for: Raspberry Pi, laptop, trying it out."
echo ""
echo "  2) Standard (recommended)"
echo "     Chatbot + Traefik for web routing + app deployment."
echo "     Sheldon can deploy and manage containers for you."
echo "     Good for: VPS, any server with Docker."
echo ""
read -p "Choice [1-2]: " choice
echo ""

case $choice in
    1)
        echo "Setting up Minimal mode..."
        mkdir -p "$SHELDON_DIR"
        curl -fsSL "$REPO_URL/deploy/minimal/run.sh" -o "$SHELDON_DIR/run.sh"
        chmod +x "$SHELDON_DIR/run.sh"

        echo ""
        echo "================================================"
        echo "  Minimal setup complete!"
        echo "================================================"
        echo ""
        echo "To start Sheldon:"
        echo ""
        echo "  cd $SHELDON_DIR"
        echo "  TELEGRAM_TOKEN=xxx ANTHROPIC_API_KEY=xxx ./run.sh"
        echo ""
        ;;
    2)
        echo "Setting up Standard mode..."
        mkdir -p "$SHELDON_DIR"
        cd "$SHELDON_DIR"

        # Download compose files
        curl -fsSL "$REPO_URL/deploy/standard/docker-compose.yml" -o docker-compose.yml
        curl -fsSL "$REPO_URL/deploy/standard/apps.yml" -o apps.yml
        curl -fsSL "$REPO_URL/deploy/standard/.env.example" -o .env.example

        # Create directories
        mkdir -p data skills letsencrypt

        # Copy env example if .env doesn't exist
        if [ ! -f .env ]; then
            cp .env.example .env
        fi

        echo ""
        echo "================================================"
        echo "  Standard setup complete!"
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
        ;;
    *)
        echo "Invalid choice. Exiting."
        exit 1
        ;;
esac
