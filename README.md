# Sheldon

A personal AI assistant that remembers your life across 14 structured domains, runs on your own infrastructure, and can write and deploy code autonomously.

## Features

- **Persistent Memory**: Graph-based memory system (sheldonmem) stores facts, entities, and relationships in SQLite
- **14 Life Domains**: Organizes knowledge across Identity, Health, Relationships, Work, Finances, Goals, and more
- **Code Generation**: Integrated Claude Code for writing, testing, and deploying applications
- **Self-Hosted**: Runs on your own k8s cluster (k3s on a cheap VPS works great)
- **Multi-Provider LLM**: Supports Claude, Kimi, and other providers
- **Telegram & Discord**: Chat interfaces with long-polling (no inbound ports needed)
- **Skills System**: Markdown-based skills for specialized tasks
- **Object Storage**: MinIO integration for file uploads and backups

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                        SHELDON                          │
│                                                         │
│   Telegram/Discord ──► Agent Loop ──► LLM Provider      │
│          │                │                             │
│          │                ├── SOUL.md (personality)     │
│          │                ├── Skills (markdown)         │
│          │                └── Tools                     │
│          │                      ├── recall_memory       │
│          │                      ├── write_code          │
│          │                      ├── deploy_app          │
│          │                      └── ...                 │
│          │                                              │
│          └──► sheldonmem (SQLite + sqlite-vec)          │
│                    ├── Entities (graph nodes)           │
│                    ├── Facts (domain-tagged knowledge)  │
│                    ├── Edges (relationships)            │
│                    └── Vectors (semantic search)        │
└─────────────────────────────────────────────────────────┘
```

## Quick Start

### Local Development

```bash
# Clone
git clone https://github.com/bowerhall/sheldon.git
cd sheldon

# Set environment variables
export TELEGRAM_TOKEN="your-telegram-bot-token"
export LLM_API_KEY="your-api-key"
export LLM_PROVIDER="claude"  # or kimi, openai, etc.
export LLM_MODEL="claude-sonnet-4-20250514"
export SHELDON_MEMORY="/tmp/sheldon.db"
export SHELDON_ESSENCE="./core/essence"

# Run
cd core && go run ./cmd/sheldon
```

### VPS Deployment (Recommended)

One-command install on a fresh Ubuntu VPS (Hetzner CX32, 8GB RAM, €8/mo recommended):

```bash
# SSH to your VPS, then:
curl -fsSL https://raw.githubusercontent.com/bowerhall/sheldon/main/deploy/scripts/vps-install.sh | sudo bash
```

Or manually:

```bash
# Clone and run install script
git clone https://github.com/bowerhall/sheldon.git /opt/sheldon
cd /opt/sheldon

# Configure secrets
cp deploy/k8s/base/secrets.yaml.example deploy/k8s/base/secrets.yaml
# Edit secrets.yaml with your credentials (TELEGRAM_TOKEN, KIMI_API_KEY, etc.)

# Run installer
sudo ./deploy/scripts/vps-install.sh
```

### Manual Kubernetes Deployment

```bash
# Create namespace
kubectl create namespace sheldon

# Copy and configure secrets
cp deploy/k8s/base/secrets.yaml.example deploy/k8s/base/secrets.yaml
# Edit secrets.yaml with your credentials

# Deploy (choose overlay based on resources)
kubectl apply -k deploy/k8s/overlays/full     # 8GB+ RAM, in-cluster Ollama
kubectl apply -k deploy/k8s/overlays/lite     # 4GB RAM, external Ollama
kubectl apply -k deploy/k8s/overlays/minimal  # 2GB RAM, no embeddings
```

See [deploy/README.md](deploy/README.md) for detailed deployment options.

## Configuration

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `TELEGRAM_TOKEN` | Telegram bot token | Yes (if using Telegram) |
| `DISCORD_TOKEN` | Discord bot token | Yes (if using Discord) |
| `LLM_PROVIDER` | LLM provider (claude, kimi, openai) | Yes |
| `LLM_API_KEY` | API key for LLM provider | Yes |
| `LLM_MODEL` | Model name | Yes |
| `SHELDON_MEMORY` | Path to SQLite database | Yes |
| `SHELDON_ESSENCE` | Path to SOUL.md directory | Yes |
| `NVIDIA_API_KEY` | NVIDIA NIM API key for coder (free at build.nvidia.com) | No |
| `KIMI_API_KEY` | Moonshot Kimi API key (fallback for coder) | No |
| `CODER_MODEL` | Model for code generation (default: kimi-k2.5) | No |
| `GIT_TOKEN` | GitHub PAT for code commits | No |
| `GIT_ORG_URL` | GitHub org URL for repos | No |

### The 14 Domains

Sheldon organizes memory across these life domains:

| ID | Domain | Layer |
|----|--------|-------|
| 1 | Identity & Self | Core |
| 2 | Body & Health | Core |
| 3 | Mind & Emotions | Inner |
| 4 | Beliefs & Worldview | Inner |
| 5 | Knowledge & Skills | Inner |
| 6 | Relationships & Social | World |
| 7 | Work & Career | World |
| 8 | Finances & Assets | World |
| 9 | Place & Environment | World |
| 10 | Goals & Aspirations | Temporal |
| 11 | Preferences & Tastes | Meta |
| 12 | Rhythms & Routines | Temporal |
| 13 | Life Events & Decisions | Temporal |
| 14 | Unconscious Patterns | Meta |

## Project Structure

```
sheldon/
├── core/                   # Main Go application
│   ├── cmd/sheldon/        # Entry point
│   └── internal/
│       ├── agent/          # Agent loop
│       ├── bot/            # Telegram/Discord
│       ├── coder/          # Claude Code integration
│       ├── config/         # Configuration
│       ├── deployer/       # K8s app deployment
│       ├── llm/            # LLM providers
│       ├── storage/        # MinIO client
│       └── tools/          # Agent tools
├── pkg/sheldonmem/         # Memory package (standalone)
├── deploy/
│   ├── k8s/                # Kubernetes manifests
│   └── docker/             # Dockerfiles
├── skills/                 # Skill definitions
└── docs/                   # Documentation
```

## Docker Images

```bash
# Main application
docker pull ghcr.io/bowerhall/sheldon:latest

# Claude Code runner (for isolated code execution)
docker pull ghcr.io/bowerhall/sheldon-claude-code:latest
```

## Development

```bash
# Build
cd core && go build -o bin/sheldon ./cmd/sheldon

# Test memory package
cd pkg/sheldonmem && go test -v

# Format
go fmt ./...
go vet ./...
```

## Security

- No inbound ports required (Telegram/Discord use long-polling)
- Network policies isolate components
- Code generation runs in isolated k8s Jobs via Ollama + NVIDIA NIM
- Credentials stored in k8s Secrets
- Output sanitization for API keys and tokens

## License

[AGPL-3.0](LICENSE)
