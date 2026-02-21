# Sheldon

A personal AI assistant that remembers your entire life, runs on your own infrastructure, and can write and deploy code autonomously.

## Features

- 🚀 **Zero-cost embeddings** — Local Ollama models, no API fees
- 🧠 **Unified memory** — SQLite + sqlite-vec, single file, no external DB
- 🔒 **Isolated coder** — Ephemeral Docker containers for safe code execution
- ⚡ **One-click deploy** — Push to GitHub → deployed on VPS (~€8/mo)
- 🗂️ **14 life domains** — Structured memory across your entire life
- ⏰ **Scheduled agent triggers** — Cron + scheduler + reminder + task runner in one
- 🏠 **Self-hosted** — Your data, your infrastructure

## Scheduled Agent Triggers

Unlike traditional heartbeat systems that just send notifications, Sheldon's cron system **wakes the full agent** with context. The agent decides what to do: send a check-in, remind you about something, or start working on a task.

```
You: "check on me every 6 hours"
Sheldon: "I'll check in with you every 6 hours."

[6 hours later]
Sheldon: "Hey! How's your afternoon going? Last we talked you were working on the API refactor."

You: "go quiet until tomorrow"
Sheldon: "Got it, I'll be quiet until tomorrow morning."
```

```
You: "remind me to take meds every evening for two weeks"
Sheldon: "I'll remind you at 8pm every evening for the next two weeks."

[8:00 PM]
Sheldon: "Time for your evening meds!"
```

```
You: "build me a weather dashboard at 3pm tomorrow"
Sheldon: "I'll start building your weather dashboard tomorrow at 3pm."

[3:00 PM next day]
Sheldon: "Starting on your weather dashboard now."
[works autonomously, uses coder tools]
Sheldon: "Done! Deployed to weather.yourdomain.com"
```

**How it works:**
1. You tell Sheldon what you want (reminder, check-in schedule, scheduled task)
2. Sheldon stores context in memory + creates a cron with a keyword
3. When cron fires → recalls memory with keyword → injects into agent loop
4. Agent takes action based on context (not just a dumb notification)

**Why this is better than traditional heartbeat:**
- 🎯 **Context-aware** — Agent knows *why* it's reaching out
- 🛠️ **Can take action** — Not just notify, but actually do work
- 🎚️ **Runtime control** — "go quiet for 3 hours" via conversation, not config
- 🔗 **Memory-linked** — Updates to facts automatically reflect in reminders

## Architecture

```
                         Internet
                            │
                            ▼ :80/:443
┌───────────────────────────────────────────────────────────────┐
│                         Traefik                                │
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
| `HEARTBEAT_CHAT_ID` | Your Telegram chat ID (for error alerts) |

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

---

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

## The 14 Life Domains

Sheldon organizes memory across structured domains:

| Layer        | Domains                                                     |
| ------------ | ----------------------------------------------------------- |
| **Core**     | Identity & Self, Body & Health                              |
| **Inner**    | Mind & Emotions, Beliefs & Worldview, Knowledge & Skills    |
| **World**    | Relationships, Work & Career, Finances, Place & Environment |
| **Temporal** | Goals & Aspirations, Rhythms & Routines, Life Events        |
| **Meta**     | Preferences & Tastes, Unconscious Patterns                  |

## License

[AGPL-3.0](LICENSE)
