#!/bin/bash
set -e

# Sheldon Installer
# Usage: curl -fsSL bowerhall.ai/sheldon/install | sudo bash

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo ""
echo "======================================"
echo "   Sheldon Installer"
echo "======================================"
echo ""

# Check root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}Run as root: sudo bash${NC}"
    exit 1
fi

# Check Ubuntu/Debian
if ! command -v apt-get &> /dev/null; then
    echo -e "${RED}This script requires Ubuntu or Debian${NC}"
    exit 1
fi

echo -e "${GREEN}[1/6]${NC} Installing Docker..."
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com | sh
else
    echo "Docker already installed"
fi

echo ""
echo -e "${GREEN}[2/6]${NC} Installing Ollama..."
if ! command -v ollama &> /dev/null; then
    curl -fsSL https://ollama.com/install.sh | sh
else
    echo "Ollama already installed"
fi

echo ""
echo -e "${GREEN}[3/6]${NC} Pulling AI models (this takes a few minutes)..."
systemctl start ollama || true
sleep 3
ollama pull nomic-embed-text
ollama pull qwen2.5:3b

echo ""
echo -e "${GREEN}[4/6]${NC} Setting up Sheldon..."
mkdir -p /opt/sheldon /data

cat > /opt/sheldon/docker-compose.yml << 'COMPOSE'
services:
  sheldon:
    image: ghcr.io/bowerhall/sheldon:latest
    container_name: sheldon
    restart: unless-stopped
    volumes:
      - /data:/data
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - DATA_DIR=/data
      - OLLAMA_HOST=http://host.docker.internal:11434
      - TELEGRAM_TOKEN=${TELEGRAM_TOKEN}
      - OWNER_CHAT_ID=${OWNER_CHAT_ID}
      - KIMI_API_KEY=${KIMI_API_KEY:-}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}
      - OPENAI_API_KEY=${OPENAI_API_KEY:-}
      - TZ=${TZ:-UTC}
      - STORAGE_ENDPOINT=${STORAGE_ENDPOINT:-http://minio:9000}
      - STORAGE_ADMIN_USER=${STORAGE_ADMIN_USER:-admin}
      - STORAGE_ADMIN_PASSWORD=${STORAGE_ADMIN_PASSWORD:-}
      - STORAGE_SHELDON_USER=${STORAGE_SHELDON_USER:-sheldon}
      - STORAGE_SHELDON_PASSWORD=${STORAGE_SHELDON_PASSWORD:-}
    extra_hosts:
      - "host.docker.internal:host-gateway"
    networks:
      - sheldon-net
    depends_on:
      - minio

  minio:
    image: minio/minio:latest
    container_name: minio
    restart: unless-stopped
    command: server /data --console-address ":9001"
    volumes:
      - /data/minio:/data
    environment:
      - MINIO_ROOT_USER=admin
      - MINIO_ROOT_PASSWORD=${STORAGE_ADMIN_PASSWORD}
    ports:
      - "9000:9000"
      - "9001:9001"
    networks:
      - sheldon-net

networks:
  sheldon-net:
    driver: bridge
COMPOSE

cat > /etc/systemd/system/sheldon.service << 'SERVICE'
[Unit]
Description=Sheldon AI Assistant
After=docker.service ollama.service
Requires=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/sheldon
EnvironmentFile=/opt/sheldon/.env
ExecStartPre=/usr/bin/docker compose pull
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down
TimeoutStartSec=300

[Install]
WantedBy=multi-user.target
SERVICE

systemctl daemon-reload
systemctl enable ollama
systemctl enable sheldon

echo ""
echo -e "${GREEN}[5/6]${NC} Configuration"
echo ""
echo "======================================"
echo ""

read -p "Telegram bot token (from @BotFather): " telegram_token
read -p "Your Telegram chat ID (from @userinfobot): " owner_chat_id

echo ""
echo "Enter at least one LLM API key:"
read -p "KIMI_API_KEY (Enter to skip): " kimi_key
read -p "ANTHROPIC_API_KEY (Enter to skip): " anthropic_key
read -p "OPENAI_API_KEY (Enter to skip): " openai_key

echo ""
detected_tz=$(timedatectl show --property=Timezone --value 2>/dev/null || cat /etc/timezone 2>/dev/null || echo "")
if [[ -n "$detected_tz" ]]; then
    read -p "Timezone [$detected_tz]: " timezone
    timezone=${timezone:-$detected_tz}
else
    read -p "Timezone (e.g., Europe/London): " timezone
    while [[ -z "$timezone" ]]; do
        echo -e "${RED}Timezone is required${NC}"
        read -p "Timezone: " timezone
    done
fi

echo ""
storage_password=$(openssl rand -base64 16 | tr -dc 'a-zA-Z0-9' | head -c 16)
echo -e "MinIO password (auto-generated): ${YELLOW}${storage_password}${NC}"
echo "Save this - you'll need it for the MinIO console."

cat > /opt/sheldon/.env << EOF
TELEGRAM_TOKEN=${telegram_token}
OWNER_CHAT_ID=${owner_chat_id}
KIMI_API_KEY=${kimi_key}
ANTHROPIC_API_KEY=${anthropic_key}
OPENAI_API_KEY=${openai_key}
TZ=${timezone}
STORAGE_ADMIN_PASSWORD=${storage_password}
STORAGE_SHELDON_PASSWORD=${storage_password}
EOF

chmod 600 /opt/sheldon/.env

echo ""
echo -e "${GREEN}[6/6]${NC} Starting Sheldon..."
systemctl start sheldon

PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || echo "your-server-ip")

echo ""
echo "======================================"
echo -e "   ${GREEN}Sheldon is running!${NC}"
echo "======================================"
echo ""
echo "Open Telegram and message your bot."
echo ""
echo "MinIO console: http://${PUBLIC_IP}:9001"
echo "  Username: admin"
echo "  Password: ${storage_password}"
echo ""
echo "Commands:"
echo "  systemctl status sheldon  - Check status"
echo "  docker logs -f sheldon    - View logs"
echo "  systemctl restart sheldon - Restart"
echo ""
