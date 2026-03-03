# Deployment Guide

## Prerequisites

- VPS (Hetzner CX32 recommended, €8/mo) or any machine with Docker
- Domain pointing to your VPS (for Standard mode)
- Telegram bot token (from @BotFather)
- Anthropic API key or Kimi API key

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/bowerhall/sheldon/main/core/deploy/install.sh | bash
```

---

## What You Get

- Sheldon chatbot via Telegram/Discord
- Memory (SQLite + sqlite-vec for semantic search)
- Local Ollama for embeddings and fact extraction (zero API cost)
- Skills framework
- Cron reminders
- Traefik for web routing
- HTTPS via Let's Encrypt
- Sheldon can deploy apps for you
- Apps accessible via subdomains

**Architecture:**

```
Internet
    │
    ▼ :80/:443
┌─────────────────────────────────────┐
│              Traefik                 │
│         (reverse proxy)             │
└──────┬──────────┬──────────┬────────┘
       │          │          │
       ▼          ▼          ▼
   Sheldon    App #1     App #2
       │
       ▼
    Ollama (embeddings + extraction)

   All containers on same Docker network.
   Traefik auto-discovers via labels.
```

**Step-by-step install:**

```bash
# 1. SSH into your VPS
ssh root@your-vps-ip

# 2. Install Docker
curl -fsSL https://get.docker.com | sh

# 3. Create Sheldon directory
mkdir -p /opt/sheldon
cd /opt/sheldon

# 4. Download compose files
curl -O https://raw.githubusercontent.com/bowerhall/sheldon/main/core/deploy/docker-compose.yml
curl -O https://raw.githubusercontent.com/bowerhall/sheldon/main/core/.env.example

# 5. Configure secrets (choose one)

# Option A: Simple .env file (quick setup)
cp .env.example .env
nano .env  # Add your tokens

# Option B: Doppler (recommended for production)
# Install: curl -sLf https://cli.doppler.com/install.sh | sh
# Setup:   doppler login && doppler setup
# Run:     doppler run -- docker compose up -d

# 6. Create network and start
docker network create sheldon-net
docker compose up -d

# 7. Verify
docker compose ps
docker compose logs -f sheldon
```

**Files:**

| File                 | Purpose                            |
| -------------------- | ---------------------------------- |
| `docker-compose.yml` | Infrastructure (Traefik + Sheldon) |
| `apps.yml`           | Sheldon-managed apps               |
| `.env`               | Configuration                      |
| `data/`              | Sheldon memory + data              |
| `skills/`            | Custom skills                      |

**Deploying apps:**

Tell Sheldon: "Deploy a simple todo API"

Sheldon will:

1. Generate the app code
2. Add it to `apps.yml`
3. Run `docker compose -f apps.yml up -d`
4. Configure Traefik labels for routing

---

## Secrets Management

Two options for managing API keys and tokens:

| Approach | Best For | Security |
|----------|----------|----------|
| `.env` file | Quick setup, single user | Keys in file on disk |
| Doppler | Production, teams | Keys in encrypted vault |

**Option A: .env file (simple)**

```bash
cp .env.example .env
nano .env  # Add your tokens
docker compose up -d
```

Keys are stored in `.env` on disk. Secure the file:
```bash
chmod 600 .env
```

**Option B: Doppler (recommended for production)**

```bash
# Install Doppler CLI
curl -sLf https://cli.doppler.com/install.sh | sh

# Login and setup
doppler login
doppler setup  # Select your project

# Run with secrets injected
doppler run -- docker compose up -d
```

Keys never touch disk - injected at runtime from Doppler's encrypted vault.

---

## No Domain? Use IP Address

If you don't have a domain, Sheldon auto-detects your server's public IP:

```bash
# No DOMAIN in .env = Sheldon uses public IP
# Apps accessible via container name on Docker network
```

Note: Without a domain, apps won't have public URLs via Traefik (subdomain routing requires DNS). Apps are still accessible within the Docker network.

---

## Auto-Deploy with GitHub Actions + Doppler

We use [Doppler](https://doppler.com) for secrets management. This gives you:

- Nice web UI to manage all secrets
- One GitHub secret instead of 20+
- Works on any cloud (Hetzner, AWS, DigitalOcean, etc.)
- Team sharing and audit logs

### 1. Setup Doppler (one-time)

1. Sign up at [doppler.com](https://doppler.com) (free tier: 5 projects, unlimited secrets)

2. Create a new project called `sheldon`

3. Add your secrets in the Doppler dashboard:

**Required:**

| Secret              | Description                                               |
| ------------------- | --------------------------------------------------------- |
| `VPS_HOST`          | Your VPS IP address                                       |
| `VPS_USER`          | SSH username (usually `root`)                             |
| `VPS_SSH_KEY`       | Your SSH private key (full key including BEGIN/END lines) |
| `GHCR_TOKEN`        | GitHub PAT with `read:packages` scope                     |
| `TELEGRAM_TOKEN`    | Telegram bot token from @BotFather                        |
| `ANTHROPIC_API_KEY` | Anthropic API key                                         |
| `DOMAIN`            | Your domain (e.g., `sheldon.example.com`)                 |

**Optional:**

| Secret               | Default     | Description                                          |
| -------------------- | ----------- | ---------------------------------------------------- |
| `ACME_EMAIL`         | -           | Email for Let's Encrypt HTTPS                        |
| `ALERT_CHAT_ID`      | -           | Telegram chat ID for budget warnings and error alerts|
| `EMBEDDER_PROVIDER`  | -           | `ollama` or `voyage`                                 |
| `EMBEDDER_BASE_URL`  | -           | Ollama URL if using                                  |
| `EMBEDDER_MODEL`     | -           | Embedding model name                                 |
| `CODER_ISOLATED`     | `true`      | Run coder in Docker containers (recommended)         |
| `CODER_PROVIDER`     | auto        | LLM provider for coder (kimi, claude, openai)        |
| `CODER_MODEL`        | auto        | Model for code generation (depends on provider)      |
| `GIT_TOKEN`          | -           | GitHub PAT for coder to push code                    |
| `GIT_USER_NAME`      | `Sheldon`   | Git commit author name                               |
| `GIT_USER_EMAIL`     | -           | Git commit author email                              |
| `GIT_ORG_URL`        | -           | GitHub org URL (e.g., `https://github.com/your-org`) |
| `TZ`                 | `UTC`       | Timezone                                             |

4. Generate a Service Token:
   - Go to `Project Settings > Service Tokens`
   - Click `Generate Service Token`
   - Name it `github-actions`
   - Copy the token

### 2. Add Doppler Token to GitHub

In your GitHub repo, go to `Settings > Secrets and variables > Actions` and add:

| Secret          | Value                         |
| --------------- | ----------------------------- |
| `DOPPLER_TOKEN` | The service token from step 4 |

That's it. Just one secret.

### 3. Deploy

Push to main:

```bash
git push origin main
```

GitHub Actions will automatically:

1. Build and push Docker images
2. Fetch secrets from Doppler
3. SSH into your VPS
4. Install Docker (if first run)
5. Create directories (if first run)
6. Generate `.env` from Doppler secrets
7. Copy `docker-compose.yml`
8. Pull images and start services

### Zero-Touch Deployment

With Doppler configured, deploying to a fresh VPS is:

1. Buy VPS, get IP and SSH key
2. Add `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY` to Doppler
3. Push to main

No SSH required. GitHub Actions handles everything including Docker installation.

---

## DNS Setup

Point your domain to your VPS:

```
A    sheldon.yourdomain.com    → your-vps-ip
A    *.sheldon.yourdomain.com  → your-vps-ip  (for deployed apps)
```

**Required subdomains:**

| Subdomain | Purpose |
|-----------|---------|
| `storage.yourdomain.com` | MinIO Console (web UI) |
| `s3.yourdomain.com` | MinIO API (for file sharing links) |

The wildcard `*` record covers these automatically. If not using wildcard, add them explicitly.

---

## HTTPS with Let's Encrypt

Edit `docker-compose.yml` and uncomment the ACME lines in the traefik service:

```yaml
command:
  # ... existing commands ...
  - '--certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=web'
  - '--certificatesresolvers.letsencrypt.acme.email=${ACME_EMAIL}'
  - '--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json'
```

Add `ACME_EMAIL` to your `.env`:

```env
ACME_EMAIL=you@example.com
```

Restart:

```bash
docker compose up -d
```

---

## Code Generation (Coder)

Coder is enabled automatically when you provide an LLM API key. It uses the same provider keys as the main agent (KIMI_API_KEY, ANTHROPIC_API_KEY, or OPENAI_API_KEY).

By default, coder runs in isolated Docker containers for security. To enable:

1. Ensure the coder sandbox image is available:

```bash
docker pull ghcr.io/bowerhall/sheldon-coder-sandbox:latest
```

2. Optionally configure provider in `.env`:

```env
CODER_PROVIDER=claude  # or kimi, openai (auto-detected if not set)
CODER_MODEL=claude-sonnet-4-20250514  # (auto-selected based on provider if not set)
```

3. Restart:

```bash
docker compose up -d
```

Coder runs in isolated mode by default. Set `CODER_ISOLATED=false` only for local development.

---

## Pinchtab (Authenticated Browsing)

Pinchtab enables Sheldon to browse sites where you're logged in (Gmail, GitHub, etc.) using persistent browser sessions.

### How It Works

1. Pinchtab runs a real Chrome browser with saved cookies/sessions
2. You log into your accounts once (manually)
3. Sheldon can then browse those sites as you
4. Each browse requires your approval via button click (security)

### Setup

1. Add to your `.env`:

```env
PINCHTAB_URL=http://pinchtab:9867
PINCHTAB_TOKEN=your-secret-token
```

2. Start with the pinchtab profile:

```bash
docker compose --profile pinchtab up -d
```

3. Log into your accounts through Pinchtab's browser (sessions persist in volume)

### Security Considerations

- **Page content goes to LLM provider** - Don't use for banking or highly sensitive sites
- **Approval required** - Sheldon can't browse authenticated sites without your button click
- **Token protected** - `PINCHTAB_TOKEN` prevents unauthorized API access
- **Isolated from Sheldon** - Runs in separate container, Sheldon only gets page text

### Usage

Once configured, Sheldon automatically uses Pinchtab for authenticated sites:

- "Check my Gmail for unread emails" → uses `browse_session`
- "Search for X on Google" → uses regular `browse` (anonymous)

---

## Commands Reference

```bash
# View logs
docker compose logs -f

# Restart
docker compose restart

# Stop
docker compose down

# Update manually
docker compose pull
docker compose up -d

# Check status
docker compose ps
```

---

## Troubleshooting

**Sheldon not responding:**

```bash
docker compose logs sheldon | tail -50
```

**Traefik not routing:**

```bash
docker compose logs traefik | tail -50
# Check dashboard at http://your-vps-ip:8080
```

**Reset everything:**

```bash
docker compose down -v
rm -rf data/
docker compose up -d
```

---

## Resource Requirements

| Config                  | RAM | Storage | CPU     | Cost  |
| ----------------------- | --- | ------- | ------- | ----- |
| Base (Sheldon + Ollama) | 2GB | 5GB     | 2 cores | €5/mo |
| With Coder              | 4GB | 10GB    | 4 cores | €8/mo |

**Recommended:** Hetzner CX32 (4 vCPU, 8GB RAM, €8.49/mo) handles everything with headroom.
