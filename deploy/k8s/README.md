# Sheldon Kubernetes Deployment

## Deployment Modes

### Full (default)
Deploys Sheldon + Ollama locally. Use when your server has enough RAM for models.

```bash
kubectl apply -k deploy/k8s/
# or explicitly:
kubectl apply -k deploy/k8s/overlays/full/
```

### Lite
Deploys Sheldon only. LLMs are accessed via remote URLs (homelab, VPS, cloud).

```bash
kubectl apply -k deploy/k8s/overlays/lite/
```

Edit `overlays/lite/config-patch.yaml` to set your remote URLs:
```yaml
data:
  EMBEDDER_URL: "http://your-homelab:11434"
  CLAUDE_CODE_BASE_URL: "http://your-homelab:11434"
```

## Structure

```
deploy/k8s/
├── kustomization.yaml      # Points to full by default
├── base/                   # Core Sheldon resources
│   ├── namespace.yaml
│   ├── sheldon.yaml
│   ├── config.yaml
│   ├── essence.yaml
│   └── secrets.yaml
└── overlays/
    ├── full/               # Sheldon + Ollama (local models)
    │   ├── ollama.yaml
    │   └── config-patch.yaml
    └── lite/               # Sheldon only (remote models)
        └── config-patch.yaml
```

## Secrets

Copy and edit the secrets file:
```bash
cp base/secrets.yaml base/secrets-local.yaml
# Edit with your actual values
```

Required secrets:
- `TELEGRAM_TOKEN` - Telegram bot token
- `KIMI_API_KEY` - Kimi API key (or other LLM provider)
- `HEARTBEAT_CHAT_ID` - Your Telegram chat ID
- `SHELDON_TIMEZONE` - Your timezone (e.g., "Europe/Berlin")

Optional:
- `CLAUDE_CODE_API_KEY` - For Claude Code bridge (if using Anthropic/OpenRouter)

## Architecture

```
LITE MODE (Pi, small VPS)          FULL MODE (homelab, beefy VPS)
┌─────────────────────┐            ┌─────────────────────┐
│  Sheldon only       │            │  Sheldon + Ollama   │
│  ~50MB RAM          │            │  ~8GB+ RAM          │
│                     │            │                     │
│  LLMs via remote:   │            │  Everything local:  │
│  - homelab          │            │  - embeddings       │
│  - other VPS        │            │  - coding models    │
│  - cloud fallback   │            │  - chat (optional)  │
└─────────────────────┘            └─────────────────────┘
```
