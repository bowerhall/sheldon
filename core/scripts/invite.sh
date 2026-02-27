#!/bin/bash
set -e

# Sheldon Homelab Invite Script
# Usage: HEADSCALE_URL=https://hs.example.com AUTHKEY=key curl bowerhall.ai/sheldon/invite | sudo bash

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration - pass as env vars
HEADSCALE_URL="${HEADSCALE_URL:-}"
AUTHKEY="${AUTHKEY:-}"
AGENT_IMAGE="${AGENT_IMAGE:-ghcr.io/bowerhall/sheldon-homelab-agent:latest}"

print_banner() {
    echo -e "${CYAN}"
    cat << "EOF"

   _____ __         __    __
  / ___// /_  ___  / /___/ /___  ____
  \__ \/ __ \/ _ \/ / __  / __ \/ __ \
 ___/ / / / /  __/ / /_/ / /_/ / / / /
/____/_/ /_/\___/_/\__,_/\____/_/ /_/

       H O M E L A B   A G E N T

EOF
    echo -e "${NC}"
}

print_step() {
    echo -e "${BLUE}==>${NC} $1"
}

print_success() {
    echo -e "${GREEN}  ✓${NC} $1"
}

print_error() {
    echo -e "${RED}  ✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}  !${NC} $1"
}

check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_error "Please run as root (sudo)"
        exit 1
    fi
}

check_required_vars() {
    if [ -z "$HEADSCALE_URL" ]; then
        print_error "HEADSCALE_URL is required"
        echo ""
        echo "Usage:"
        echo "  HEADSCALE_URL=https://hs.example.com AUTHKEY=your-key curl ... | sudo bash"
        echo ""
        echo "Get an authkey from your Sheldon VPS:"
        echo "  docker exec headscale headscale preauthkeys create --user 1 --expiration 1h"
        exit 1
    fi

    if [ -z "$AUTHKEY" ]; then
        print_error "AUTHKEY is required"
        echo ""
        echo "Get an authkey from your Sheldon VPS:"
        echo "  docker exec headscale headscale preauthkeys create --user 1 --expiration 1h"
        exit 1
    fi
}

check_command() {
    if command -v "$1" &> /dev/null; then
        print_success "$1 is installed"
        return 0
    else
        return 1
    fi
}

install_docker() {
    print_step "Installing Docker..."

    if check_command docker; then
        return 0
    fi

    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
    print_success "Docker installed"
}

install_tailscale() {
    print_step "Installing Tailscale..."

    if check_command tailscale; then
        return 0
    fi

    curl -fsSL https://tailscale.com/install.sh | sh
    print_success "Tailscale installed"
}

get_machine_name() {
    print_step "Machine Configuration"
    echo ""
    echo -e "${CYAN}  What should Sheldon call this machine?${NC}"
    echo -e "  Examples: gpu-beast, homelab, living-room-server"
    echo ""
    read -p "  Machine name: " MACHINE_NAME

    if [ -z "$MACHINE_NAME" ]; then
        MACHINE_NAME=$(hostname)
        print_warning "Using hostname: $MACHINE_NAME"
    fi
}

join_network() {
    print_step "Joining Sheldon's network..."

    tailscale up --login-server="$HEADSCALE_URL" --authkey="$AUTHKEY" --hostname="$MACHINE_NAME"

    TAILSCALE_IP=$(tailscale ip -4 2>/dev/null || echo "pending")
    print_success "Joined network as $MACHINE_NAME ($TAILSCALE_IP)"
}

start_agent() {
    print_step "Starting homelab-agent..."

    # Stop existing container if running
    docker rm -f homelab-agent 2>/dev/null || true

    # Run the agent
    docker run -d \
        --name homelab-agent \
        --restart unless-stopped \
        -p 8080:8080 \
        -v /var/run/docker.sock:/var/run/docker.sock \
        "$AGENT_IMAGE"

    print_success "homelab-agent started on port 8080"
}

start_ollama() {
    print_step "Setting up Ollama..."

    if docker ps -a --format '{{.Names}}' | grep -q '^ollama$'; then
        print_success "Ollama container already exists"
        docker start ollama 2>/dev/null || true
    else
        print_step "Creating Ollama container..."
        docker run -d \
            --name ollama \
            --restart unless-stopped \
            -p 11434:11434 \
            -v ollama_data:/root/.ollama \
            ollama/ollama
        print_success "Ollama container created"
    fi
}

print_summary() {
    TAILSCALE_IP=$(tailscale ip -4 2>/dev/null || echo "unknown")

    echo ""
    echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}                    SETUP COMPLETE!                          ${NC}"
    echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  Machine name:     ${CYAN}$MACHINE_NAME${NC}"
    echo -e "  Tailscale IP:     ${CYAN}$TAILSCALE_IP${NC}"
    echo -e "  Agent status:     ${GREEN}running${NC}"
    echo ""
    echo -e "  ${YELLOW}Tell Sheldon:${NC}"
    echo -e "  \"I added a new machine called $MACHINE_NAME\""
    echo ""
    echo -e "  ${YELLOW}To switch Ollama to this machine:${NC}"
    echo -e "  \"Switch ollama to $MACHINE_NAME\""
    echo ""
    echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo ""
}

main() {
    print_banner

    check_root
    check_required_vars

    echo ""
    print_step "Checking dependencies..."

    install_docker
    install_tailscale

    echo ""
    get_machine_name

    echo ""
    join_network

    echo ""
    start_ollama
    start_agent

    print_summary
}

main "$@"
