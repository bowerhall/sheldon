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

while true; do
    show_config
    echo "  r) Restart Sheldon"
    echo "  s) Show logs"
    echo "  q) Quit"
    echo ""
    read -p "Select option to edit (1-7) or action (r/s/q): " choice

    case $choice in
        1)
            read -p "New Telegram Token: " new_val
            [[ -n "$new_val" ]] && TELEGRAM_TOKEN="$new_val" && save_config
            ;;
        2)
            read -p "New Owner Chat ID: " new_val
            [[ -n "$new_val" ]] && OWNER_CHAT_ID="$new_val" && save_config
            ;;
        3)
            detected_tz=$(timedatectl show --property=Timezone --value 2>/dev/null || cat /etc/timezone 2>/dev/null || echo "")
            read -p "New Timezone [$detected_tz]: " new_val
            new_val=${new_val:-$detected_tz}
            [[ -n "$new_val" ]] && TZ="$new_val" && save_config
            ;;
        4)
            read -p "New KIMI API Key (or 'clear' to remove): " new_val
            if [[ "$new_val" == "clear" ]]; then
                KIMI_API_KEY="" && save_config
            elif [[ -n "$new_val" ]]; then
                KIMI_API_KEY="$new_val" && save_config
            fi
            ;;
        5)
            read -p "New Anthropic API Key (or 'clear' to remove): " new_val
            if [[ "$new_val" == "clear" ]]; then
                ANTHROPIC_API_KEY="" && save_config
            elif [[ -n "$new_val" ]]; then
                ANTHROPIC_API_KEY="$new_val" && save_config
            fi
            ;;
        6)
            read -p "New OpenAI API Key (or 'clear' to remove): " new_val
            if [[ "$new_val" == "clear" ]]; then
                OPENAI_API_KEY="" && save_config
            elif [[ -n "$new_val" ]]; then
                OPENAI_API_KEY="$new_val" && save_config
            fi
            ;;
        7)
            read -p "New MinIO Password: " new_val
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
