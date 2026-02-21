# Sheldon

A personal AI assistant that remembers your entire life, runs on your own infrastructure, and can write and deploy code autonomously.

## Why Sheldon?

🚀 **Zero-cost embeddings & extraction** — Local Ollama models for semantic search and fact extraction. No per-request API fees.

🧠 **Unified memory system** — SQLite + sqlite-vec in a single file. Entities, facts, relationships, vectors. No external databases.

🔒 **Isolated code execution** — Coder runs in ephemeral Docker containers. Write, test, deploy apps safely.

⚡ **One-click VPS deploy** — Push to GitHub, Doppler handles secrets, Actions deploys. ~€8/month on Hetzner.

🗂️ **14 life domains** — Structured memory across Identity, Health, Relationships, Work, Finances, Goals, and more.

💬 **Proactive check-ins** — Scheduled heartbeats with memory-augmented context. Sheldon reaches out when relevant.

🏠 **Self-hosted** — Your data, your infrastructure. No cloud dependency for core functionality.

```
You: "remind me to take meds every evening for two weeks"
Sheldon: "Got it! I'll remind you about your meds every evening at 8pm."

[8:00 PM]
Sheldon: "Time to take your meds"
```

## Architecture

```
                         Internet
                            │
                            ▼ :80/:443
┌───────────────────────────────────────────────────────────────┐
│                         Traefik                               │
│                    (reverse proxy + HTTPS)                    │
└─────────────┬─────────────────────────────────┬───────────────┘
              │                                 │
              ▼                                 ▼
┌─────────────────────────┐         ┌───────────────────────────┐
│        Sheldon          │         │      Your Apps            │
│                         │         │   (deployed by Sheldon)   │
│  Telegram ──► Agent     │         └───────────────────────────┘
│               │         │
│               ▼         │         ┌───────────────────────────┐
│         ┌─────────┐     │         │         Ollama            │
│         │ Tools   │     │◄───────►│  - nomic-embed-text       │
│         └────┬────┘     │         │  - qwen2:0.5b             │
│              │          │         │  (embeddings + extraction)│
│              ▼          │         └───────────────────────────┘
│     ┌──────────────┐    │
│     │  sheldonmem  │    │         ┌───────────────────────────┐
│     │   (SQLite)   │    │         │    Coder Sandbox          │
│     │              │    │────────►│  (ephemeral containers)   │
│     │ • Entities   │    │         │  ollama launch claude     │
│     │ • Facts      │    │         └───────────────────────────┘
│     │ • Vectors    │    │
│     └──────────────┘    │
└─────────────────────────┘

All containers on sheldon-net. Single VPS. ~€8/month.
```

## Deploy to VPS (5 minutes)

### Prerequisites

- Hetzner account (or any VPS provider)
- GitHub account
- Telegram bot token (from @BotFather)
- Kimi API key (from platform.moonshot.cn)

### 1. Fork & Clone

```bash
git clone https://github.com/YOUR_USERNAME/sheldon.git
cd sheldon
```

### 2. Create VPS

1. Go to [console.hetzner.cloud](https://console.hetzner.cloud)
2. Create project → Add Server
3. **Image**: Ubuntu 24.04
4. **Type**: CX33 (4 vCPU, 8GB RAM, €8.49/mo)
5. **SSH Key**: Add your public key
6. Create and note the IP address

### 3. Setup Doppler (Secrets Manager)

1. Sign up at [doppler.com](https://doppler.com) (free tier)
2. Create project: `sheldon`
3. Add secrets:

**Required:**
| Secret | Value |
|--------|-------|
| `VPS_HOST` | Your VPS IP |
| `VPS_USER` | `root` |
| `VPS_SSH_KEY` | Your SSH private key (full content) |
| `GHCR_TOKEN` | GitHub PAT with `write:packages` scope |
| `TELEGRAM_TOKEN` | From @BotFather |
| `KIMI_API_KEY` | From Moonshot |
| `TZ` | Your timezone (e.g., `UTC`) |

**Optional:**
| Secret | Description |
|--------|-------------|
| `LLM_PROVIDER` | `kimi`, `claude`, or `openai` |
| `ANTHROPIC_API_KEY` | If using Claude |
| `GIT_TOKEN` | GitHub PAT for coder to push code |
| `GIT_ORG_URL` | e.g., `https://github.com/your-org` |
| `HEARTBEAT_ENABLED` | `true` for proactive check-ins |
| `HEARTBEAT_CHAT_ID` | Your Telegram chat ID |

4. Generate Service Token: Project Settings → Service Tokens → Generate
5. Copy the token (starts with `dp.st.`)

### 4. Add Doppler Token to GitHub

1. Your repo → Settings → Secrets and variables → Actions
2. New repository secret:
   - Name: `DOPPLER_TOKEN`
   - Value: paste the service token

### 5. Deploy

```bash
git push origin main
```

GitHub Actions will automatically:
- Build and push Docker images
- SSH into your VPS
- Install Docker (first run)
- Deploy Sheldon + Ollama + Traefik

Watch progress: `https://github.com/YOUR_USERNAME/sheldon/actions`

### 6. Message Your Bot

Open Telegram, find your bot, send a message. Sheldon is live.

## The 14 Domains

| Layer | Domains |
|-------|---------|
| **Core** | Identity & Self, Body & Health |
| **Inner** | Mind & Emotions, Beliefs & Worldview, Knowledge & Skills |
| **World** | Relationships, Work & Career, Finances, Place & Environment |
| **Temporal** | Goals & Aspirations, Rhythms & Routines, Life Events |
| **Meta** | Preferences & Tastes, Unconscious Patterns |

## Local Development

```bash
cd sheldon

# Copy env and fill in values
cp core/.env.example core/.env

# Run
cd core && go run ./cmd/sheldon
```

## Project Structure

```
sheldon/
├── core/                    # Main Go application
│   ├── cmd/sheldon/         # Entry point
│   ├── internal/            # Agent, bot, coder, tools
│   ├── essence/             # SOUL.md, IDENTITY.md
│   └── deploy/              # Docker Compose, Dockerfiles
├── pkg/sheldonmem/          # Memory package (SQLite + sqlite-vec)
├── skills/                  # Markdown skill definitions
└── docs/                    # Documentation
```

## License

[AGPL-3.0](LICENSE)
