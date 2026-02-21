#!/bin/bash
# Minimal Sheldon setup - just the chatbot, no app deployment
set -e

DATA_DIR="${SHELDON_DATA:-$HOME/.sheldon}"
mkdir -p "$DATA_DIR"

# Check for required env vars
if [ -z "$TELEGRAM_TOKEN" ]; then
    echo "Error: TELEGRAM_TOKEN is required"
    echo "Usage: TELEGRAM_TOKEN=xxx ANTHROPIC_API_KEY=xxx ./run.sh"
    exit 1
fi

if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "Error: ANTHROPIC_API_KEY is required"
    exit 1
fi

echo "Starting Sheldon (minimal mode)..."
echo "Data directory: $DATA_DIR"

docker run -d \
    --name sheldon \
    --restart unless-stopped \
    -v "$DATA_DIR:/data" \
    -e TELEGRAM_TOKEN="$TELEGRAM_TOKEN" \
    -e ANTHROPIC_API_KEY="$ANTHROPIC_API_KEY" \
    -e MEMORY_PATH="/data/sheldon.db" \
    -e ESSENCE_PATH="/app/essence" \
    ${SHELDON_IMAGE:-ghcr.io/bowerhall/sheldon:latest}

echo "Sheldon is running!"
echo "Send a message to your Telegram bot to start chatting."
