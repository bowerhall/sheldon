#!/bin/bash
set -e

# Sheldon Config Manager
# Usage: curl -fsSL bowerhall.ai/sheldon/config | sudo bash

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

ENV_FILE="/opt/sheldon/.env"

echo ""
echo -e "${GREEN}"
cat << "EOF"
   _____ __         __    __
  / ___// /_  ___  / /___/ /___  ____
  \__ \/ __ \/ _ \/ / __  / __ \/ __ \
 ___/ / / / /  __/ / /_/ / /_/ / / / /
/____/_/ /_/\___/_/\__,_/\____/_/ /_/

      C O N F I G   M A N A G E R
EOF
echo -e "${NC}"

# Check root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}Run as root: sudo bash${NC}"
    exit 1
fi

# Check if installed
if [[ ! -f "$ENV_FILE" ]]; then
    echo -e "${RED}Sheldon not installed. Run the installer first:${NC}"
    echo "curl -fsSL bowerhall.ai/sheldon/install | sudo bash"
    exit 1
fi

# Load current config
source "$ENV_FILE"

mask_key() {
    local key="$1"
    if [[ -z "$key" ]]; then
        echo "(not set)"
    elif [[ ${#key} -gt 8 ]]; then
        echo "${key:0:4}...${key: -4}"
    else
        echo "****"
    fi
}

show_config() {
    echo -e "${CYAN}Current Configuration:${NC}"
    echo ""
    echo "  1) Telegram Token:    $(mask_key "$TELEGRAM_TOKEN")"
    echo "  2) Owner Chat ID:     ${OWNER_CHAT_ID:-(not set)}"
    echo "  3) Timezone:          ${TZ:-UTC}"
    echo ""
    echo "  4) KIMI API Key:      $(mask_key "$KIMI_API_KEY")"
    echo "  5) Anthropic API Key: $(mask_key "$ANTHROPIC_API_KEY")"
    echo "  6) OpenAI API Key:    $(mask_key "$OPENAI_API_KEY")"
    echo ""
    echo "  7) MinIO Password:    $(mask_key "$STORAGE_ADMIN_PASSWORD")"
    echo ""
}

save_config() {
    cat > "$ENV_FILE" << EOF
TELEGRAM_TOKEN=${TELEGRAM_TOKEN}
OWNER_CHAT_ID=${OWNER_CHAT_ID}
KIMI_API_KEY=${KIMI_API_KEY}
ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
OPENAI_API_KEY=${OPENAI_API_KEY}
TZ=${TZ}
STORAGE_ADMIN_PASSWORD=${STORAGE_ADMIN_PASSWORD}
STORAGE_SHELDON_PASSWORD=${STORAGE_SHELDON_PASSWORD}
EOF
    chmod 600 "$ENV_FILE"
}

show_status() {
    echo -e "${CYAN}Service Status:${NC}"
    echo ""
    docker compose -f /opt/sheldon/docker-compose.yml ps --format "table {{.Name}}\t{{.Status}}" 2>/dev/null || docker ps --filter "name=sheldon" --format "table {{.Names}}\t{{.Status}}"
    echo ""
}

while true; do
    show_config
    echo "  r) Restart Sheldon"
    echo "  s) Show logs"
    echo "  u) Update Sheldon"
    echo "  p) Status (all services)"
    echo "  m) Current model"
    echo "  c) Cleanup (prune Docker)"
    echo "  d) Disk usage"
    echo "  b) Backup memory"
    echo "  q) Quit"
    echo ""
    read -p "Select option (1-7/r/s/u/p/m/c/d/b/q): " choice < /dev/tty

    case $choice in
        1)
            read -p "New Telegram Token: " new_val < /dev/tty
            [[ -n "$new_val" ]] && TELEGRAM_TOKEN="$new_val" && save_config
            ;;
        2)
            read -p "New Owner Chat ID: " new_val < /dev/tty
            [[ -n "$new_val" ]] && OWNER_CHAT_ID="$new_val" && save_config
            ;;
        3)
            detected_tz=$(timedatectl show --property=Timezone --value 2>/dev/null || cat /etc/timezone 2>/dev/null || echo "")
            read -p "New Timezone [$detected_tz]: " new_val < /dev/tty
            new_val=${new_val:-$detected_tz}
            [[ -n "$new_val" ]] && TZ="$new_val" && save_config
            ;;
        4)
            read -p "New KIMI API Key (or 'clear' to remove): " new_val < /dev/tty
            if [[ "$new_val" == "clear" ]]; then
                KIMI_API_KEY="" && save_config
            elif [[ -n "$new_val" ]]; then
                KIMI_API_KEY="$new_val" && save_config
            fi
            ;;
        5)
            read -p "New Anthropic API Key (or 'clear' to remove): " new_val < /dev/tty
            if [[ "$new_val" == "clear" ]]; then
                ANTHROPIC_API_KEY="" && save_config
            elif [[ -n "$new_val" ]]; then
                ANTHROPIC_API_KEY="$new_val" && save_config
            fi
            ;;
        6)
            read -p "New OpenAI API Key (or 'clear' to remove): " new_val < /dev/tty
            if [[ "$new_val" == "clear" ]]; then
                OPENAI_API_KEY="" && save_config
            elif [[ -n "$new_val" ]]; then
                OPENAI_API_KEY="$new_val" && save_config
            fi
            ;;
        7)
            read -p "New MinIO Password: " new_val < /dev/tty
            if [[ -n "$new_val" ]]; then
                STORAGE_ADMIN_PASSWORD="$new_val"
                STORAGE_SHELDON_PASSWORD="$new_val"
                save_config
                echo -e "${YELLOW}Note: Also update MinIO container with new password${NC}"
            fi
            ;;
        r|R)
            echo ""
            echo -e "${GREEN}Restarting Sheldon...${NC}"
            systemctl restart sheldon
            echo -e "${GREEN}Done!${NC}"
            echo ""
            ;;
        s|S)
            echo ""
            docker logs --tail 50 sheldon
            echo ""
            ;;
        u|U)
            echo ""
            echo -e "${GREEN}Updating Sheldon...${NC}"
            cd /opt/sheldon
            docker compose pull sheldon
            docker compose up -d sheldon
            echo -e "${GREEN}Update complete!${NC}"
            echo ""
            ;;
        p|P)
            echo ""
            show_status
            ;;
        m|M)
            echo ""
            echo -e "${CYAN}Current Model Config:${NC}"
            echo ""
            docker exec sheldon cat /data/runtime_config.json 2>/dev/null || echo "Using env defaults"
            echo ""
            ;;
        c|C)
            echo ""
            echo -e "${YELLOW}This will remove unused Docker images and containers.${NC}"
            read -p "Continue? (y/N): " confirm < /dev/tty
            if [[ "$confirm" =~ ^[Yy]$ ]]; then
                echo -e "${GREEN}Cleaning up...${NC}"
                docker system prune -f
                echo -e "${GREEN}Done!${NC}"
            fi
            echo ""
            ;;
        d|D)
            echo ""
            echo -e "${CYAN}Disk Usage:${NC}"
            echo ""
            df -h /opt/sheldon 2>/dev/null || df -h /
            echo ""
            echo -e "${CYAN}Sheldon Data:${NC}"
            du -sh /opt/sheldon/data/* 2>/dev/null || echo "  No data directory"
            echo ""
            echo -e "${CYAN}Docker Usage:${NC}"
            docker system df
            echo ""
            ;;
        b|B)
            echo ""
            BACKUP_FILE="/opt/sheldon/backups/sheldon_$(date +%Y%m%d_%H%M%S).db"
            mkdir -p /opt/sheldon/backups
            echo -e "${GREEN}Creating backup...${NC}"
            cp /opt/sheldon/data/sheldon.db "$BACKUP_FILE"
            echo -e "${GREEN}Backup saved: $BACKUP_FILE${NC}"
            echo ""
            ls -lh /opt/sheldon/backups/*.db 2>/dev/null | tail -5
            echo ""
            ;;
        q|Q)
            echo ""
            echo "Bye!"
            exit 0
            ;;
        *)
            echo -e "${RED}Invalid option${NC}"
            ;;
    esac

    echo ""
done
