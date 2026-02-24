# Sheldon vs OpenClaw

## Model Switching

### OpenClaw

```bash
/model claude-sonnet-4-20250514
/model openai/gpt-5.1-codex
/model list
/model status
```

Or CLI:
```bash
openclaw models set anthropic/claude-opus-4-6
```

### Sheldon

```
"Switch to Claude Sonnet"
"What model are you using?"
"List available models"
```

Tools: `switch_model`, `current_model`, `list_models`, `list_providers`

### Comparison

| Feature | OpenClaw | Sheldon |
|---------|----------|---------|
| Switch via chat | `/model <name>` | Natural language |
| Switch via CLI | `openclaw models set` | Edit runtime_config.json |
| Hot reload | Yes | Yes (next message) |
| Multi-provider | Yes | Yes (Claude, OpenAI, Kimi, Ollama) |
| Pull local models | Via Ollama separately | `pull_model` tool |
| Model capabilities | Manual config | Auto-detected (vision, video, tools) |
| Persist across restart | Config file | runtime_config.json |

Both can do it. OpenClaw uses slash commands, Sheldon uses natural language + tools.

---

## Pulling Local Models

### OpenClaw

```bash
# You run this yourself in terminal
ollama pull llama3.3
ollama pull qwen2.5-coder:32b

# Then tell OpenClaw to use it
/model ollama/llama3.3
```

OpenClaw discovers models from Ollama but **can't pull them** - you do that outside the chat.

### Sheldon

```
You: "Pull llama3.3 for me"
Sheldon: "Pulling llama3.3... done (4.7GB)"

You: "What local models do I have?"
Sheldon: [calls list_models] "You have: nomic-embed-text, qwen2.5:3b, llama3.3"

You: "Remove qwen, I don't need it anymore"
Sheldon: "Removed qwen2.5:3b"
```

Tools: `pull_model`, `remove_model`, `list_models`

### Comparison

| Action | OpenClaw | Sheldon |
|--------|----------|---------|
| Pull model | Terminal: `ollama pull x` | Chat: "pull x" |
| List models | Terminal: `ollama list` | Chat: "what models?" |
| Remove model | Terminal: `ollama rm x` | Chat: "remove x" |
| Switch to model | Chat: `/model ollama/x` | Chat: "use x" |
| Auto-discover | Yes | Yes |

Sheldon keeps you in the conversation. OpenClaw makes you context-switch to terminal for model management.

---

## Heartbeat & Proactive Systems

### OpenClaw

**Heartbeat:**
```
Every 30 min → Read HEARTBEAT.md → Check tasks → HEARTBEAT_OK or message
```

**Cron:**
```yaml
# In config file
jobs:
  - id: email-check
    schedule: "0 */30 9-21 * * *"
    prompt: "Check my email"
```

Static file-based. Agent reads `HEARTBEAT.md` for instructions, returns `HEARTBEAT_OK` if nothing to do.

### Sheldon

**Memory-Triggered Crons:**
```
You: "Check on me every 6 hours"
Sheldon: [creates cron with keyword "check-in"]

6 hours later...
Cron fires → Memory recall "check-in" → Gets context about user →
Sheldon: "Hey! How's the project going? Last time you mentioned being stuck on the API integration."
```

The cron **recalls memory** when it fires, so Sheldon knows WHY it's reaching out.

### Comparison

| Feature | OpenClaw | Sheldon |
|---------|----------|---------|
| Setup | Config file + HEARTBEAT.md | "Remind me to..." |
| Context source | Static markdown file | Memory recall |
| Fires with | Fixed prompt | Keyword → recalled facts |
| Feels like | Robot on a timer | Friend who remembers |
| One-time reminders | Cron that self-deletes | `one_time: true` |
| Pause temporarily | Delete and recreate | `pause_cron` tool |

### Example: Check-in Flow

**OpenClaw:**
```markdown
# HEARTBEAT.md
Check inbox, calendar, and pending tasks.
If anything urgent, message me.
Otherwise return HEARTBEAT_OK.
```
Same check every time, no memory of past conversations.

**Sheldon:**
```
You: "Hey, I've been stressed about the deadline. Check on me every few hours."
Sheldon: [saves fact: user stressed about deadline, creates cron]

Later...
Cron fires → Recalls: "user stressed about deadline" →
Sheldon: "How's the deadline looking? Need help with anything?"
```
Contextual, remembers what you talked about.

### The Difference

OpenClaw is systematic: configured intervals, checklist-based, returns OK/not-OK.

Sheldon is relational: remembers why it's checking, adapts message to context, feels like a friend who noticed you've been quiet.

---

## Memory

### OpenClaw

```
Session starts → Context window fills up → Pruning kicks in → Old messages gone
```

Memory is the conversation history. When it gets too long, older messages are dropped. No persistence between sessions unless you configure external memory plugins.

### Sheldon

```
You: "I moved to Berlin last month"
Sheldon: [saves fact: user lives in Berlin, domain: Place & Environment]

3 months later...
You: "What's the weather like where I live?"
Sheldon: [recalls: user lives in Berlin] "Let me check Berlin weather..."
```

**Architecture:**
- SQLite + sqlite-vec for vector search
- Graph structure: entities → facts → edges
- 14 life domains (identity, health, relationships, work, finances, etc.)
- Hybrid retrieval: keyword matching + semantic similarity
- Contradiction detection: new facts automatically supersede old ones
- Confidence scoring with decay over time

### Comparison

| Feature | OpenClaw | Sheldon |
|---------|----------|---------|
| Storage | In-memory | SQLite + vectors |
| Structure | Flat messages | Graph (entities, facts, edges) |
| Persistence | Session only | Lifetime |
| Semantic search | No | Yes (Ollama embeddings) |
| Contradictions | Manual | Auto-detected and superseded |
| Domains | None | 14 life domains |
| Recall | Scroll up | `recall_memory` tool |

### Example: Contradiction Handling

**OpenClaw:**
```
March: "I live in NYC"
June: "I moved to Berlin"
September: "Where do I live?"
→ Both facts in context, model has to figure it out (or old one pruned)
```

**Sheldon:**
```
March: [saves: city = NYC, confidence 0.9]
June: [saves: city = Berlin, confidence 0.9] → auto-supersedes NYC fact
September: "Where do I live?"
→ [recalls: city = Berlin] "You live in Berlin"
```

---

## Code to Deploy

### OpenClaw

```
You: "Build me a weather dashboard"
OpenClaw: [writes code to local files]
OpenClaw: "I've created the files. Run `npm start` to test it."

You: *opens terminal, runs npm start, tests locally*
You: *figures out deployment yourself*
```

OpenClaw writes code. Deployment is your problem.

### Sheldon

```
You: "Build me a weather dashboard"
Sheldon: [calls write_code → spawns coder sandbox]
Sheldon: [coder writes React app, commits to git]
Sheldon: [calls deploy_app → builds Docker image → pushes to registry]
Sheldon: [Traefik picks up new service, provisions TLS cert]
Sheldon: "Done. Live at https://weather.yourdomain.com"
```

Tools: `write_code`, `build_image`, `deploy_app`, `remove_app`, `list_apps`, `app_status`, `app_logs`

### Comparison

| Step | OpenClaw | Sheldon |
|------|----------|---------|
| Write code | Yes | Yes (sandboxed coder) |
| Test locally | You do it | Coder sandbox |
| Build image | No | `build_image` |
| Deploy | No | `deploy_app` |
| TLS certs | No | Traefik + Let's Encrypt |
| Live URL | No | `https://app.yourdomain.com` |
| View logs | No | `app_logs` |
| Rollback | No | Redeploy previous image |

### The Pipeline

```
Request → Coder Sandbox → Git Commit → Docker Build → Registry Push → Compose Deploy → Traefik Routes → Live
    ↑                                                                                              ↓
    └──────────────────────────── All within one conversation ─────────────────────────────────────┘
```

---

## Security

### OpenClaw

- Docker socket mounted directly
- API keys in config files (readable)
- Network: standard Docker networking
- Containers run with full access

### Sheldon

- Docker socket behind proxy (limited operations)
- API keys in env vars only (not runtime-writable)
- Network: private Headscale mesh with ACLs
- Coder runs in isolated sandbox (no network access to host)
- Git tokens held by orchestrator, not coder

### Comparison

| Layer | OpenClaw | Sheldon |
|-------|----------|---------|
| Docker access | Direct socket | Socket proxy (filtered) |
| Secrets | Config file | Env-only |
| Network | Open | Headscale + ACLs |
| Code execution | Direct | Sandboxed container |
| Git credentials | In environment | Orchestrator only |

### Coder Isolation

```
┌─────────────────────────────────────────┐
│ Sheldon (orchestrator)                  │
│   - Has GIT_TOKEN                       │
│   - Clones repo to workspace            │
│   - Spawns coder container              │
└───────────────┬─────────────────────────┘
                │ volume mount (no token)
                ▼
┌─────────────────────────────────────────┐
│ Coder Sandbox                           │
│   - Writes code                         │
│   - Cannot access GIT_TOKEN             │
│   - Cannot push directly                │
└───────────────┬─────────────────────────┘
                │ returns
                ▼
┌─────────────────────────────────────────┐
│ Sheldon (orchestrator)                  │
│   - Reviews changes                     │
│   - Pushes to branch                    │
└─────────────────────────────────────────┘
```

---

## File & Storage

### OpenClaw

```
You: "Save this document"
OpenClaw: [writes to local filesystem]
OpenClaw: "Saved to ~/documents/note.md"

You: "Send me that file"
→ Not possible via chat
```

Files live on local disk. Sharing requires you to access them yourself.

### Sheldon

```
You: "Save this document"
Sheldon: [uploads to MinIO] "Saved to documents/note.md"

You: "Send me a link to that file"
Sheldon: [generates presigned URL] "Here: https://s3.yourdomain.com/... (expires in 7 days)"

You: "Download that PDF from this URL and save it"
Sheldon: [fetches URL, uploads to MinIO] "Saved."

You: "Backup your memory"
Sheldon: [exports SQLite, uploads to backups bucket] "Memory backed up."
```

Tools: `upload_file`, `download_file`, `list_files`, `delete_file`, `share_link`, `fetch_url`, `backup_memory`

### Comparison

| Feature | OpenClaw | Sheldon |
|---------|----------|---------|
| Storage | Local filesystem | MinIO (S3-compatible) |
| Share files | Manual | `share_link` (presigned URLs) |
| Fetch from URL | No | `fetch_url` (up to 100MB) |
| Organized buckets | No | user, agent, backups |
| Memory backup | No | `backup_memory` |
| Access from anywhere | No | Yes (MinIO has web UI) |

---

## Homelab & Networking

### OpenClaw

No built-in infrastructure management. You set up your own reverse proxy, VPN, storage, etc.

### Sheldon

```
You: "Add my GPU server to the network"
Sheldon: "Run this on your GPU server:"
         curl -fsSL https://... | AUTHKEY=xxx bash

*GPU server joins Headscale mesh*

You: "What containers are running on gpu-server?"
Sheldon: [calls list_containers on gpu-server] "ollama, prometheus, grafana"

You: "Restart ollama on gpu-server"
Sheldon: [calls restart_container] "Done."

You: "Use the GPU server for inference"
*Set OLLAMA_HOST to gpu-server's Headscale IP*
```

### Components

| Component | Purpose |
|-----------|---------|
| Headscale | Self-hosted Tailscale (private WireGuard mesh) |
| Traefik | Reverse proxy with automatic Let's Encrypt |
| MinIO | S3-compatible object storage |
| Homelab Agent | Remote container management API |
| Docker Socket Proxy | Filtered Docker access |

### Your Own Tailscale

Sheldon's Headscale isn't just for servers - it replaces Tailscale for all your devices:

```bash
# On any device
tailscale up --login-server=https://hs.yourdomain.com
```

Now your laptop, phone, NAS, and servers are all on a private mesh:
- `ssh laptop.sheldon.local`
- `smb://nas.sheldon.local`
- Access home services from anywhere
- No ports exposed to internet

### Comparison

| Feature | OpenClaw | Sheldon |
|---------|----------|---------|
| Private mesh VPN | No | Headscale |
| Remote container management | No | Homelab agent |
| Reverse proxy | External | Traefik built-in |
| TLS certificates | External | Let's Encrypt auto |
| Object storage | No | MinIO |
| Multi-machine | No | Invite script |
| Personal devices | No | Full Tailscale replacement |

---

## Skills

### OpenClaw

```
# Install from marketplace
/skill install weather-checker
/skill install smart-home-control

# 5700+ community skills
# JSON schema + JS/TS implementation
# Published to central registry
```

Large ecosystem, but skills are third-party code running with full access.

### Sheldon

```
You: "Here's a skill I found: https://github.com/user/skills/raw/main/POMODORO.md"
Sheldon: [fetches URL, shows content]
You: "Looks good, install it"
Sheldon: [saves to skills directory] "Installed."

Or:

You: "Search for productivity skills for personal assistants"
Sheldon: [browses web, finds skill files]
Sheldon: "Found these: [links]. Want me to show you any?"
You: "Show me the first one"
Sheldon: [fetches and displays content]
You: "Install it"
```

Tools: `install_skill`, `list_skills`, `read_skill`, `remove_skill`, `use_skill`, `save_skill`

### Comparison

| Feature | OpenClaw | Sheldon |
|---------|----------|---------|
| Format | JSON + JS/TS | Markdown |
| Source | Central marketplace | Any URL |
| Install | `/skill install x` | "Install this URL" |
| Verification | Trust the marketplace | Read before install |
| Discovery | Marketplace search | Web search |
| Create | Publish to registry | Just write markdown |

### The Difference

OpenClaw has a bigger ecosystem but you're trusting third-party code.

Sheldon has fewer skills but you verify each one. You can read the markdown, understand what it does, then install. Or ask Sheldon to browse the web for skills and review them together.

---

## Vision & Media

### OpenClaw

```
# Supports vision models
/model gpt-4o
*send image*
OpenClaw: "I can see a cat in this image"

# Video via media-understanding subsystem
scripts/analyze-video.sh video.mp4 --prompt "Summarize"
```

Vision works in chat. Video analysis uses a separate subsystem with Gemini models.

### Sheldon

```
You: *send image* "What's in this?"
Sheldon: "This is a receipt from Whole Foods totaling $47.82"

You: *send image* "Save this to my receipts folder"
Sheldon: [uploads to MinIO] "Saved to receipts/2024-02-24.jpg"

You: "Send me that receipt I saved"
Sheldon: [downloads from MinIO, sends photo] *image appears*

You: *send video* "Summarize this"
Sheldon: "This is a 2-minute tutorial on..."
```

### Comparison

| Feature | OpenClaw | Sheldon |
|---------|----------|---------|
| Receive images | Yes | Yes |
| Analyze images | Yes | Yes |
| Video analysis | Yes (Gemini, separate subsystem) | Yes (Claude, inline) |
| Save to storage | No | MinIO upload |
| Send images back | No | `send_image` |
| Capability detection | Manual config | Auto-detected per model |

### The Difference

Both handle images and video. OpenClaw routes video to a separate media-understanding subsystem using Gemini. Sheldon processes video inline in the conversation using Claude's native video support.

Sheldon also auto-detects capabilities per model:

```
Claude Opus → vision: true, video: true
GPT-4o → vision: true, video: false
Kimi → vision: false
Ollama llava → vision: true
```

If you send an image to a non-vision model, Sheldon tells you instead of failing silently.
