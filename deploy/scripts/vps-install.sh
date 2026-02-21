#!/bin/bash
set -euo pipefail

# Sheldon VPS Auto-Install Script
# Installs k3s, deploys Sheldon with full overlay (in-cluster Ollama)
# Requires: Ubuntu 22.04+, 8GB RAM minimum (Hetzner CX32 recommended)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
error() { echo -e "${RED}[x]${NC} $1"; exit 1; }

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    error "This script must be run as root"
fi

# Check minimum RAM (8GB)
TOTAL_RAM=$(free -g | awk '/^Mem:/{print $2}')
if [[ $TOTAL_RAM -lt 7 ]]; then
    warn "Less than 8GB RAM detected ($TOTAL_RAM GB). Sheldon may not run optimally."
    read -p "Continue anyway? [y/N] " -n 1 -r
    echo
    [[ ! $REPLY =~ ^[Yy]$ ]] && exit 1
fi

log "Starting Sheldon VPS installation..."

# Install prerequisites
log "Installing prerequisites..."
apt-get update
apt-get install -y curl git jq

# Install k3s
if command -v k3s &> /dev/null; then
    log "k3s already installed, skipping..."
else
    log "Installing k3s..."
    curl -sfL https://get.k3s.io | sh -s - --disable traefik

    # Wait for k3s to be ready
    log "Waiting for k3s to be ready..."
    sleep 10
    until kubectl get nodes &>/dev/null; do
        sleep 5
    done
fi

# Set up kubectl for non-root user
SUDO_USER_HOME=$(eval echo ~${SUDO_USER:-$USER})
mkdir -p "$SUDO_USER_HOME/.kube"
cp /etc/rancher/k3s/k3s.yaml "$SUDO_USER_HOME/.kube/config"
chown -R ${SUDO_USER:-$USER}:${SUDO_USER:-$USER} "$SUDO_USER_HOME/.kube"
chmod 600 "$SUDO_USER_HOME/.kube/config"

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

log "k3s is ready"
kubectl get nodes

# Clone or update repository
REPO_DIR="/opt/sheldon"
if [[ -d "$REPO_DIR" ]]; then
    log "Updating repository..."
    cd "$REPO_DIR"
    git pull
else
    log "Cloning repository..."
    git clone https://github.com/bowerhall/sheldon.git "$REPO_DIR"
    cd "$REPO_DIR"
fi

# Create namespace
log "Creating sheldon namespace..."
kubectl create namespace sheldon --dry-run=client -o yaml | kubectl apply -f -

# Check for secrets
SECRETS_FILE="$REPO_DIR/deploy/k8s/base/secrets.yaml"
if [[ ! -f "$SECRETS_FILE" ]]; then
    warn "Secrets file not found!"
    echo ""
    echo "Please create $SECRETS_FILE with your credentials:"
    echo ""
    cat "$REPO_DIR/deploy/k8s/base/secrets.yaml.example"
    echo ""
    echo "Required secrets:"
    echo "  - TELEGRAM_TOKEN: Your Telegram bot token (from @BotFather)"
    echo "  - KIMI_API_KEY: Moonshot Kimi API key (from platform.moonshot.cn)"
    echo "  - NVIDIA_API_KEY: NVIDIA NIM API key (from build.nvidia.com, free)"
    echo "  - MINIO_SECRET_KEY: Random password for MinIO storage"
    echo "  - GIT_TOKEN: GitHub PAT for code commits (optional)"
    echo ""
    echo "After creating secrets.yaml, run this script again."
    exit 1
fi

# Create persistent volume directories
log "Creating persistent volume directories..."
mkdir -p /data/sheldon/{memory,essence,skills,minio}
chmod 777 /data/sheldon/*

# Apply persistent volumes
log "Applying persistent volumes..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolume
metadata:
  name: sheldon-memory-pv
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: /data/sheldon/memory
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: sheldon-essence-pv
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: /data/sheldon/essence
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: sheldon-skills-pv
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: /data/sheldon/skills
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: minio-pv
spec:
  capacity:
    storage: 50Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: /data/sheldon/minio
EOF

# Deploy with kustomize (full overlay)
log "Deploying Sheldon with full overlay..."
kubectl apply -k "$REPO_DIR/deploy/k8s/overlays/full"

# Wait for pods to be ready
log "Waiting for pods to be ready..."
kubectl -n sheldon rollout status deployment/sheldon --timeout=300s || true
kubectl -n sheldon rollout status deployment/ollama --timeout=300s || true

# Show status
log "Deployment complete!"
echo ""
kubectl -n sheldon get pods
echo ""

log "Sheldon is now running!"
echo ""
echo "Useful commands:"
echo "  kubectl -n sheldon logs -f deployment/sheldon     # View Sheldon logs"
echo "  kubectl -n sheldon logs -f deployment/ollama      # View Ollama logs"
echo "  kubectl -n sheldon get pods                        # Check pod status"
echo "  kubectl -n sheldon describe pod <pod-name>         # Debug pod issues"
echo ""
echo "To update Sheldon:"
echo "  cd $REPO_DIR && git pull"
echo "  kubectl apply -k deploy/k8s/overlays/full"
echo "  kubectl -n sheldon rollout restart deployment/sheldon"
echo ""
