#!/bin/bash
set -e

# Sheldon Network Connect
# Usage: HEADSCALE_URL=https://hs.example.com AUTHKEY=your-key curl -fsSL ... | bash
# Works on: macOS, Linux

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

HEADSCALE_URL="${HEADSCALE_URL:-}"
AUTHKEY="${AUTHKEY:-}"

echo -e "${CYAN}"
cat << "EOF"
   _____ __         __    __
  / ___// /_  ___  / /___/ /___  ____
  \__ \/ __ \/ _ \/ / __  / __ \/ __ \
 ___/ / / / /  __/ / /_/ / /_/ / / / /
/____/_/ /_/\___/_/\__,_/\____/_/ /_/

         N E T W O R K
EOF
echo -e "${NC}"

if [ -z "$HEADSCALE_URL" ] || [ -z "$AUTHKEY" ]; then
    echo -e "${RED}Error: HEADSCALE_URL and AUTHKEY required${NC}"
    echo ""
    echo "Usage:"
    echo "  HEADSCALE_URL=https://hs.example.com AUTHKEY=your-key \\"
    echo "  curl -fsSL https://raw.githubusercontent.com/bowerhall/sheldon/main/core/scripts/connect.sh | bash"
    exit 1
fi

# Detect OS
OS="$(uname -s)"

# Install Tailscale
if ! command -v tailscale &> /dev/null; then
    echo "Installing Tailscale..."
    case "$OS" in
        Darwin)
            if command -v brew &> /dev/null; then
                brew install tailscale
            else
                echo -e "${RED}Please install Homebrew first: https://brew.sh${NC}"
                exit 1
            fi
            ;;
        Linux)
            curl -fsSL https://tailscale.com/install.sh | sh
            ;;
        *)
            echo -e "${RED}Unsupported OS: $OS${NC}"
            exit 1
            ;;
    esac
fi

echo -e "${GREEN}✓${NC} Tailscale installed"

# Connect to Headscale
echo "Connecting to Sheldon network..."
tailscale up --login-server="$HEADSCALE_URL" --authkey="$AUTHKEY"

TAILSCALE_IP=$(tailscale ip -4 2>/dev/null || echo "pending")

echo ""
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo -e "${GREEN}           CONNECTED!                   ${NC}"
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo ""
echo -e "  Tailscale IP: ${CYAN}$TAILSCALE_IP${NC}"
echo ""
echo "  You now have access to:"
echo "  - traefik.bowerhall.dev (dashboard)"
echo "  - All devices on Sheldon's network"
echo ""
