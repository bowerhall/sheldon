# Sheldon Deployment

## Overlays

Choose an overlay based on your VPS resources:

| Overlay | VPS Size | RAM | Cost | Features |
|---------|----------|-----|------|----------|
| `minimal` | CX21 | 4GB | ~€4/mo | External APIs only, no local models |
| `lite` | CX32 | 8GB | ~€8/mo | External LLM + local Ollama embedder |
| `full` | CX42 | 16GB | ~€15/mo | Local Ollama for embeddings + more headroom |

## Quick Deploy

```bash
# Choose your overlay
kubectl apply -k overlays/minimal  # Small VPS
kubectl apply -k overlays/lite     # Medium VPS (recommended)
kubectl apply -k overlays/full     # Large VPS
```

## Prerequisites

1. k3s or k8s cluster running
2. kubectl configured
3. Secrets configured (see below)

## Secrets

Before deploying, update `base/secrets.yaml`:

```yaml
stringData:
  TELEGRAM_TOKEN: "your-telegram-bot-token"
  LLM_API_KEY: "your-llm-api-key"
  CODER_API_KEY: "your-anthropic-key"  # for Claude Code
```

And `base/minio.yaml`:

```yaml
stringData:
  root-user: "your-minio-user"
  root-password: "your-secure-password"
```

## Resource Allocation

Each overlay sets appropriate resource limits:

### Minimal (4GB RAM)
- Sheldon: 128-256MB
- MinIO: 128-256MB
- Storage: 5GB MinIO, 1GB data

### Lite (8GB RAM)
- Sheldon: 256-512MB
- MinIO: 256-512MB
- Ollama: 1-2GB
- Storage: 10GB MinIO, 5GB data

### Full (16GB RAM)
- Sheldon: 512MB-1GB
- MinIO: 256-512MB
- Ollama: 2-4GB
- Storage: 20GB MinIO, 10GB data

## Components

- **Sheldon**: Main application
- **MinIO**: Object storage for backups
- **Ollama**: Local embeddings (lite/full only)
- **Backup CronJob**: Daily SQLite backups to MinIO

## Network Policies

Enabled by default:
- Default deny all traffic
- Sheldon: egress to APIs, internal services
- Ollama: ingress from Sheldon only
- MinIO: ingress from Sheldon namespace only
