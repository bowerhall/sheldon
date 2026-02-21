# Sheldon

A personal AI assistant that remembers your life across 14 structured domains, runs on your own infrastructure, and can write and deploy code autonomously.

## Features

- **Persistent Memory**: Graph-based memory system (sheldonmem) stores facts, entities, and relationships in SQLite
- **14 Life Domains**: Organizes knowledge across Identity, Health, Relationships, Work, Finances, Goals, and more
- **Code Generation**: Integrated Coder for writing, testing, and deploying applications
- **Self-Hosted**: Runs on your own VPS with Docker Compose + Traefik
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

We use Doppler for secrets management. Zero-touch deployment:

1. Sign up at [doppler.com](https://doppler.com) (free tier)
2. Create project `sheldon`, add your secrets
3. Add `DOPPLER_TOKEN` to GitHub Secrets
4. Push to main

GitHub Actions will automatically provision your VPS, install Docker, and deploy.

See [docs/deployment.md](docs/deployment.md) for detailed setup.

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
│       ├── coder/          # Code generation
│       ├── config/         # Configuration
│       ├── deployer/       # Docker Compose deployment
│       ├── llm/            # LLM providers
│       ├── storage/        # MinIO client
│       └── tools/          # Agent tools
├── pkg/sheldonmem/         # Memory package (standalone)
├── deploy/
│   └── docker/             # Docker Compose configs
├── skills/                 # Skill definitions
└── docs/                   # Documentation
```

## Docker Images

```bash
# Main application
docker pull ghcr.io/bowerhall/sheldon:latest

# Coder sandbox (for isolated code execution)
docker pull ghcr.io/bowerhall/sheldon-coder-sandbox:latest
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
- Network isolation via Docker Compose networks
- Code generation runs in ephemeral Docker containers
- Credentials stored in .env file (not in repo)
- Output sanitization for API keys and tokens

## License

[AGPL-3.0](LICENSE)
