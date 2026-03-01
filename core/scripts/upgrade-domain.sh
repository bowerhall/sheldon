#!/bin/bash
set -e

# Sheldon Domain Upgrade Script
# Upgrades from simple (IP-only) install to full domain setup with HTTPS
# Usage: curl -fsSL bowerhall.ai/sheldon/upgrade-domain | sudo bash

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

REPO_URL="https://raw.githubusercontent.com/bowerhall/sheldon/main"
INSTALL_DIR="/opt/sheldon"

echo ""
echo -e "${GREEN}"
cat << "EOF"
   _____ __         __    __
  / ___// /_  ___  / /___/ /___  ____
  \__ \/ __ \/ _ \/ / __  / __ \/ __ \
 ___/ / / / /  __/ / /_/ / /_/ / / / /
/____/_/ /_/\___/_/\__,_/\____/_/ /_/

    D O M A I N   U P G R A D E
EOF
echo -e "${NC}"

# Check root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}Run as root: sudo bash${NC}"
    exit 1
fi

# Check if Sheldon is installed
if [[ ! -f "$INSTALL_DIR/.env" ]]; then
    echo -e "${RED}Sheldon not installed. Run the installer first.${NC}"
    exit 1
fi

# Check if already using full compose (has traefik)
if grep -q "traefik" "$INSTALL_DIR/docker-compose.yml" 2>/dev/null; then
    echo -e "${YELLOW}Already using full domain setup.${NC}"
    echo "To change domain, edit $INSTALL_DIR/.env and restart."
    exit 0
fi

echo -e "${CYAN}This will upgrade your Sheldon to use a custom domain with HTTPS.${NC}"
echo ""
echo "Prerequisites:"
echo "  1. A domain name you own"
echo "  2. DNS configured: point your domain AND *.domain to this server's IP"
echo ""

# Get current IP
PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || curl -s api.ipify.org 2>/dev/null || echo "unknown")
echo -e "Your server IP: ${GREEN}$PUBLIC_IP${NC}"
echo ""

read -p "Your domain (e.g., sheldon.example.com): " domain < /dev/tty
while [[ -z "$domain" ]]; do
    echo -e "${RED}Domain is required${NC}"
    read -p "Your domain: " domain < /dev/tty
done

read -p "Email for Let's Encrypt (for SSL cert expiry notices): " acme_email < /dev/tty
while [[ -z "$acme_email" ]]; do
    echo -e "${RED}Email is required for HTTPS certificates${NC}"
    read -p "Email: " acme_email < /dev/tty
done

echo ""
echo -e "${YELLOW}Before continuing, make sure DNS is configured:${NC}"
echo ""
echo "  $domain        ->  $PUBLIC_IP"
echo "  *.$domain      ->  $PUBLIC_IP  (wildcard for apps)"
echo ""
echo "  Or individual records:"
echo "  hs.$domain     ->  $PUBLIC_IP  (Headscale)"
echo "  storage.$domain ->  $PUBLIC_IP  (MinIO console)"
echo ""
read -p "DNS configured? (y/N): " dns_ready < /dev/tty
if [[ ! "$dns_ready" =~ ^[Yy]$ ]]; then
    echo ""
    echo "Set up DNS first, then run this script again."
    exit 0
fi

echo ""
echo -e "${GREEN}[1/4]${NC} Backing up current config..."
cp "$INSTALL_DIR/docker-compose.yml" "$INSTALL_DIR/docker-compose.simple.yml.backup"
cp "$INSTALL_DIR/.env" "$INSTALL_DIR/.env.backup"
echo "Backup saved to $INSTALL_DIR/*.backup"

echo ""
echo -e "${GREEN}[2/4]${NC} Downloading full docker-compose..."
curl -fsSL "$REPO_URL/core/deploy/docker-compose.yml" -o "$INSTALL_DIR/docker-compose.yml"

# Download headscale config
mkdir -p "$INSTALL_DIR/headscale"
curl -fsSL "$REPO_URL/core/deploy/headscale/config.yaml" -o "$INSTALL_DIR/headscale/config.yaml"
sed -i "s/DOMAIN_PLACEHOLDER/$domain/g" "$INSTALL_DIR/headscale/config.yaml"

# Create traefik directory
mkdir -p "$INSTALL_DIR/traefik" "$INSTALL_DIR/letsencrypt"

echo ""
echo -e "${GREEN}[3/4]${NC} Updating configuration..."

# Add domain settings to .env
if ! grep -q "^DOMAIN=" "$INSTALL_DIR/.env"; then
    echo "" >> "$INSTALL_DIR/.env"
    echo "# Domain settings (added by upgrade script)" >> "$INSTALL_DIR/.env"
    echo "DOMAIN=$domain" >> "$INSTALL_DIR/.env"
    echo "ACME_EMAIL=$acme_email" >> "$INSTALL_DIR/.env"
else
    sed -i "s/^DOMAIN=.*/DOMAIN=$domain/" "$INSTALL_DIR/.env"
    if grep -q "^ACME_EMAIL=" "$INSTALL_DIR/.env"; then
        sed -i "s/^ACME_EMAIL=.*/ACME_EMAIL=$acme_email/" "$INSTALL_DIR/.env"
    else
        echo "ACME_EMAIL=$acme_email" >> "$INSTALL_DIR/.env"
    fi
fi

echo ""
echo -e "${GREEN}[4/4]${NC} Restarting Sheldon with domain support..."

cd "$INSTALL_DIR"
docker compose down
docker compose pull traefik headscale
docker compose up -d

echo ""
echo "Waiting for services..."
sleep 10

# Check if traefik is running
if docker ps --filter "name=traefik" --filter "status=running" | grep -q traefik; then
    echo -e "${GREEN}Traefik is running${NC}"
else
    echo -e "${YELLOW}Traefik may still be starting...${NC}"
fi

echo ""
echo "======================================"
echo -e "   ${GREEN}Domain upgrade complete!${NC}"
echo "======================================"
echo ""
echo "Your Sheldon is now available at:"
echo ""
echo "  Apps:          https://appname.$domain"
echo "  MinIO Console: https://storage.$domain"
echo "  Headscale:     https://hs.$domain"
echo ""
echo "HTTPS certificates will be automatically provisioned by Let's Encrypt."
echo "First request may be slow while certs are issued."
echo ""
echo "To rollback: cp $INSTALL_DIR/docker-compose.simple.yml.backup $INSTALL_DIR/docker-compose.yml"
echo ""
