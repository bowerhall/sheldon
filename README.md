# Sheldon

A personal AI assistant that remembers your entire life, runs on your own infrastructure, and can write and deploy code autonomously.

## Features

- 🚀 **Local-first memory** — Ollama embeddings on your hardware, ~$0.01/day
- 🧠 **Portable database** — SQLite + sqlite-vec, single file, salience-aware decay
- 🔒 **Isolated coder** — Ephemeral Docker containers for safe code execution
- 🌐 **Browser automation** — Sandboxed agent-browser for JS-heavy sites
- 🔐 **Authenticated browsing** — Browse your Gmail, GitHub with persistent sessions (via Pinchtab)
- ⚡ **One-click deploy** — Push to GitHub → deployed on VPS (~€8/mo)
- 🗂️ **14 life domains** — Structured memory across your entire life
- ⏰ **Context-aware triggers** — Recalls context and takes action, not dumb rigid crons
- 🏠 **Self-hosted** — Your data, your infrastructure

## What Sheldon Can Do

**Remembers things you may forget**
- "What was the name of that book I wanted to read?"
- "When did I last visit the dentist?"
- Builds a knowledge graph across 14 life domains

**Build and deploy apps with you**
- "Build me a bookmark manager and deploy it to bookmarks.mydomain.com"
- "Add dark mode to the app we made last week"
- Creates branches and PRs, you review and merge

**Browse and research**
- "Find coworking spaces in Tokyo with 24/7 access"
- "Summarize the reviews for this product"
- "Check my Gmail for unread emails" (with Pinchtab)
- Full browser automation, handles JS-heavy sites

**Remind and check in**
- "Remind me to take meds every evening"
- "Check on me every 6 hours while I'm deep in this project"
- Context-aware, not just dumb notifications

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
- ⏱️ **One-time or recurring** — "remind me at 3pm" auto-deletes after firing

## Multi-Machine / Homelab

Run Sheldon across multiple machines with private networking. Use a beefy GPU at home for Ollama while Sheldon runs on a cheap VPS.

```
┌───────────────────────────────────────────────────┐
│              Private Network (Headscale)          │
│                                                   │
│   VPS (€8/mo)              Home GPU               │
│   ┌─────────────┐         ┌─────────────┐         │
│   │   Sheldon   │◄───────►│   Ollama    │         │
│   │  Headscale  │         │    Agent    │         │
│   │   Ollama    │         └─────────────┘         │
│   │   MinIO     │                                 │
│   └─────────────┘                                 │
└───────────────────────────────────────────────────┘
```

**Add a machine:**

```bash
# 1. On your Sheldon VPS, generate a key (expires in 1 hour)
docker exec headscale headscale preauthkeys create --user 1 --expiration 1h

# 2. On the new machine, run with that key
HEADSCALE_URL=https://hs.yourdomain.com AUTHKEY=your-key \
  curl bowerhall.ai/sheldon/invite | sudo bash

# Or agent only (no private networking, just container management)
curl bowerhall.ai/sheldon/agent | sudo bash
```

**Point Ollama to your GPU server:**

Set `OLLAMA_HOST` in Doppler to your GPU server's Headscale IP:
```
OLLAMA_HOST=http://100.64.0.5:11434
```

**Manage containers remotely:**

```
"Restart ollama on gpu-server"
"Show minio logs"
"List containers on nas"
```

See [docs/homelab.md](docs/homelab.md) for full setup guide.

---

## Storage & Backups

S3-compatible storage via MinIO for files, backups, and sharing. Access the console at `https://storage.yourdomain.com` with your admin credentials.

**Security:** Sheldon has a limited user that can only access `sheldon-*` buckets. Create a `private` bucket for files Sheldon can't see.

**Store and retrieve files:**
```
"Save this as notes/ideas.md"
"Download my resume from storage"
```

**Share with expiring links:**
```
"Give me a download link for report.pdf that expires in 24 hours"
```

**Backup memory:**
```
"Backup your memory"
→ Creates timestamped backup in MinIO
```

**Archive web content:**
```
"Download https://example.com/doc.pdf and save it"
```

---

## Privacy & Sensitive Data

Sheldon can store sensitive facts (personal info you don't want exposed) that are protected in public contexts.

**Mark facts as sensitive:**
```
"My salary is $85k, mark that sensitive"
"Remember I'm seeing a therapist - keep that private"
```

**Protection layers:**

| Context | Sensitive Facts |
|---------|-----------------|
| Telegram with `OWNER_CHAT_ID` | Accessible (you're the owner) |
| Discord DM with `DISCORD_OWNER_ID` | Accessible (you're the owner) |
| Discord `DISCORD_TRUSTED_CHANNEL` | Accessible (private channel) |
| Discord other channels | Hidden (SafeMode) |
| Web browsing (isolated mode) | Hidden + recall tool blocked |

**Discord setup for privacy:**
```env
DISCORD_GUILD_ID=123...        # Who can talk to Sheldon
DISCORD_OWNER_ID=456...        # Your user ID - DMs get full access
# OR
DISCORD_TRUSTED_CHANNEL=789... # Private channel with full access
```

If neither `DISCORD_OWNER_ID` nor `DISCORD_TRUSTED_CHANNEL` is set, all conversations have full access (backwards compatible).

**Get your Discord IDs:**
- User ID: Settings → Advanced → Developer Mode → right-click yourself → Copy User ID
- Channel ID: Right-click channel → Copy Channel ID
- Server ID: Right-click server → Copy Server ID

---

## Web Interfaces

| Service | URL | Purpose |
|---------|-----|---------|
| MinIO Console | `https://storage.yourdomain.com` | File browser, buckets, share links |
| Traefik Dashboard | `https://traefik.yourdomain.com` | Routes, services, TLS certs |
| Deployed Apps | `https://appname.yourdomain.com` | Apps Sheldon deploys for you |
| Headscale | CLI only | Private mesh network management |

---

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
│         └────┬────┘     │         │    (embeddings)           │
│              │          │         │                           │
│              ▼          │         └───────────────────────────┘
│     ┌──────────────┐    │
│     │  sheldonmem  │    │         ┌───────────────────────────┐
│     │   (SQLite)   │    │         │    Coder Sandbox          │
│     │              │    │────────►│  (ephemeral containers)   │
│     │ • Entities   │    │         │  Claude Code + LLM        │
│     │ • Facts      │    │         └───────────────────────────┘
│     │ • Vectors    │    │
│     │ • Convo buf  │    │         ┌───────────────────────────┐
│     └──────────────┘    │────────►│    Browser Sandbox        │
│                         │         │  (agent-browser + Chrome) │
└─────────────────────────┘         └───────────────────────────┘

All containers on sheldon-net. Single VPS. ~€8/month.
```

## Quick Install

One command on any Ubuntu VPS (DigitalOcean, Hetzner, etc.):

```bash
curl -fsSL bowerhall.ai/sheldon/install | sudo bash
```

**What it does:**
1. Installs Docker + Ollama
2. Pulls AI models for memory
3. Sets up MinIO for file storage
4. Prompts for your Telegram token and API keys
5. Starts Sheldon

**Requirements:**
- Ubuntu 22.04+ VPS (4GB+ RAM recommended)
- Telegram bot token (free from [@BotFather](https://t.me/BotFather))
- One LLM API key (Kimi, Claude, or OpenAI)

**Recommended VPS:** Hetzner CX22 (~$5/mo) or DigitalOcean s-2vcpu-4gb ($24/mo)

---

## Deploy to VPS (5 minutes)

### Prerequisites

- VPS (Hetzner, DigitalOcean, etc.)
- GitHub account
- Telegram bot token (@BotFather)
- One LLM API key (Kimi, Anthropic, or OpenAI)
- Domain *(optional, for HTTPS)*

### 1. Fork & Clone

```bash
git clone https://github.com/YOUR_USERNAME/sheldon.git
cd sheldon
```

### 2. Create VPS

Any VPS provider works (Hetzner, DigitalOcean, Linode, Vultr, etc.):

1. Create an Ubuntu 24.04 server (4GB+ RAM recommended)
2. Add your SSH public key
3. Note the IP address

**Recommended:** Hetzner CX22 (~€4/mo) or CX33 (~€8/mo) for best value.

### 3. Configure Secrets

Choose **one** of these options:

#### Option A: GitHub Secrets Only (Simple)

Add these secrets directly in GitHub (repo → Settings → Secrets → Actions):

| Secret | Required? | Purpose |
|--------|-----------|---------|
| **Deployment** |||
| `VPS_HOST` | Yes | Your VPS IP |
| `VPS_USER` | Yes | `root` |
| `VPS_SSH_KEY` | Yes | Your SSH private key (full content) |
| `GHCR_TOKEN` | Yes | GitHub PAT with `write:packages` scope |
| **Bot Tokens** |||
| `TELEGRAM_TOKEN` | Yes* | From @BotFather |
| `DISCORD_TOKEN` | Yes* | From Discord Developer Portal |
| **LLM** |||
| `KIMI_API_KEY` | Yes** | Kimi API key |
| `ANTHROPIC_API_KEY` | Yes** | Claude API key |
| `OPENAI_API_KEY` | Yes** | OpenAI API key |
| **Storage** |||
| `STORAGE_ADMIN_PASSWORD` | Yes | MinIO console password |
| `STORAGE_SHELDON_PASSWORD` | Yes | Sheldon's MinIO password |
| **Telegram IDs** |||
| `OWNER_CHAT_ID` | No | Your Telegram user ID (enables owner-only features) |
| **Discord IDs** |||
| `DISCORD_GUILD_ID` | No | Server ID (restricts which server can talk to Sheldon) |
| `DISCORD_OWNER_ID` | No | Your Discord user ID (DMs get full access) |
| `DISCORD_TRUSTED_CHANNEL` | No | Channel ID with full access (alternative to owner ID) |
| **Optional** |||
| `TZ` | No | Your timezone (e.g., `Europe/London`) |
| `DOMAIN` | No | Your domain (enables HTTPS) |
| `ACME_EMAIL` | No | Email for Let's Encrypt (required with DOMAIN) |
| `GIT_TOKEN` | No | GitHub PAT for code push (enables coder git) |
| `GIT_ORG_URL` | No | e.g., `https://github.com/you` (required with GIT_TOKEN) |

\* At least one bot token required (Telegram or Discord)
\** At least one LLM API key required

**Getting your IDs:**
- **Telegram user ID**: Message [@userinfobot](https://t.me/userinfobot) or [@RawDataBot](https://t.me/RawDataBot)
- **Discord IDs**: Settings → Advanced → Developer Mode → right-click → Copy ID

#### Option B: Doppler (easier to manage)

1. Sign up at [doppler.com](https://doppler.com) (free tier)
2. Create project: `sheldon`
3. Add the same secrets listed above in Doppler
4. Generate Service Token: Project Settings → Service Tokens → Generate
5. Add **one** secret to GitHub:
   - Name: `DOPPLER_TOKEN`
   - Value: paste the service token (starts with `dp.st.`)

*With Doppler, you only need one GitHub secret. All other secrets are fetched from Doppler at deploy time.*

---

*Sheldon can switch LLM/coder models at runtime. Add more API keys later (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`) and Sheldon will use them.*

### 4. Setup DNS *(optional, if using DOMAIN)*

Add a wildcard record pointing to your VPS:

```
A    *.yourdomain.com    → YOUR_VPS_IP
```

This enables `storage.`, `hs.`, and any apps Sheldon deploys.

### 5. Deploy

```bash
git push origin main
```

GitHub Actions will build images, SSH into your VPS, install Docker, and deploy everything.

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

## Model Management

Sheldon uses a unified provider system for all LLM needs. Add API keys to Doppler, redeploy once, then switch freely at runtime.

**Core providers:**

| Provider | Env Key | Models |
|----------|---------|--------|
| Kimi | `KIMI_API_KEY` | kimi-k2-0711-preview, kimi-k2.5:cloud |
| Claude | `ANTHROPIC_API_KEY` | claude-sonnet-4-20250514, claude-opus-4-5-20251101 |
| OpenAI | `OPENAI_API_KEY` | gpt-4o, gpt-4o-mini |
| Ollama | - | Any local model (llama3.2, qwen2.5, etc.) |

**OpenAI-compatible providers** (add key to enable):

| Provider | Env Key |
|----------|---------|
| Mistral | `MISTRAL_API_KEY` |
| Groq | `GROQ_API_KEY` |
| Together | `TOGETHER_API_KEY` |
| DeepSeek | `DEEPSEEK_API_KEY` |
| Fireworks | `FIREWORKS_API_KEY` |
| Perplexity | `PERPLEXITY_API_KEY` |

**Switch models at runtime:**
```
"Switch to claude"
"Use gpt-4o for the coder"
"List available models"
"List providers"
"Pull llama3.2"
"Remove the unused model"
```

**Components that use models:**
- `llm` - Main chat (switchable at runtime)
- `coder` - Code generation (switchable at runtime)
- `embedder` - Embeddings (configurable via env, locked at runtime)

*Note: embedder is set via `EMBEDDER_PROVIDER` and `EMBEDDER_MODEL`. Cannot be changed at runtime — would break vector compatibility with existing memories.*

### Provider Fallback

If a provider's credits are exhausted, Sheldon auto-switches to the next available:

**kimi** → **claude** → **openai** → **ollama** (installed models only)

Ollama is last resort and only uses already-installed models (never pulls new ones). Customize preference:
```
OLLAMA_FALLBACK_MODELS=llama3.2,qwen2.5:7b,mistral
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

## Telemetry

Sheldon sends a single anonymous ping on startup to track active installs. This helps understand adoption and prioritize development.

**What's sent:**
- Anonymous install ID (SHA256 hash, not reversible)
- Version number
- Operating system

**What's NOT sent:**
- Any personal data, conversations, or memories
- IP addresses are not logged server-side

**Opt out:**
```bash
TELEMETRY_DISABLED=true
```

## License

[AGPL-3.0](LICENSE)

