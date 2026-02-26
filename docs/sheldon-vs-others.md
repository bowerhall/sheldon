# Sheldon vs Other AI Assistants

## Model Switching

### Other Assistants

```bash
/model claude-sonnet-4-20250514
/model openai/gpt-5.1-codex
/model list
/model status
```

Or CLI:
```bash
assistant models set anthropic/claude-opus-4-6
```

### Sheldon

```
"Switch to Claude Sonnet"
"What model are you using?"
"List available models"
```

Tools: `switch_model`, `current_model`, `list_models`, `list_providers`

### Comparison

| Feature | Other Assistants | Sheldon |
|---------|----------|---------|
| Switch via chat | `/model <name>` | Natural language |
| Switch via CLI | `assistant models set` | Edit runtime_config.json |
| Hot reload | Yes | Yes (next message) |
| Multi-provider | Yes | Yes (Claude, OpenAI, Kimi, Ollama) |
| Pull local models | Via Ollama separately | `pull_model` tool |
| Model capabilities | Manual config | Auto-detected (vision, video, tools) |
| Persist across restart | Config file | runtime_config.json |

Both can do it. Most assistants use slash commands, Sheldon uses natural language + tools.

---

## Pulling Local Models

### Other Assistants

```bash
# You run this yourself in terminal
ollama pull llama3.3
ollama pull qwen2.5-coder:32b

# Then switch to it
/model ollama/llama3.3
```

Most assistants discover models from Ollama but **can't pull them** - you do that outside the chat.

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

| Action | Other Assistants | Sheldon |
|--------|----------|---------|
| Pull model | Terminal: `ollama pull x` | Chat: "pull x" |
| List models | Terminal: `ollama list` | Chat: "what models?" |
| Remove model | Terminal: `ollama rm x` | Chat: "remove x" |
| Switch to model | Chat: `/model ollama/x` | Chat: "use x" |
| Auto-discover | Yes | Yes |

Sheldon keeps you in the conversation. Most assistants make you context-switch to terminal for model management.

---

## Heartbeat & Proactive Systems

### Other Assistants

**Heartbeat (config-only):**
```json
{
  "heartbeat": {
    "every": "30m"
  }
}
```

Every 30 min → Read `HEARTBEAT.md` checklist → Process tasks → Report status or message.

Heartbeat typically runs with **full session context** (can be 100k-200k tokens per run). Some communities recommend disabling native heartbeat and using isolated cron jobs instead for cost savings.

**Cron (CLI or conversational skill):**
```bash
assistant cron add --name "Morning brief" --cron "0 7 * * *" --session isolated
```

Or via a conversational skill:
```
You: "Remind me in 20 minutes"
→ Agent builds CLI command internally
```

The skill translates natural language to CLI commands. When cron fires, it executes the saved prompt.

### Sheldon

**Memory-Triggered Crons:**
```
You: "Check on me every 6 hours"
Sheldon: [creates cron with keyword "check-in"]

6 hours later...
Cron fires → Memory recall "check-in" → Gets CURRENT context about user →
Sheldon: "Hey! How's the project going? Last time you mentioned being stuck on the API integration."
```

The keyword IS a memory query, not a message. When the cron fires, Sheldon recalls facts matching that keyword and decides what to say based on current state.

### Comparison

| Feature | Other Assistants | Sheldon |
|---------|----------|---------|
| Heartbeat config | JSON file | Conversational |
| Cron setup | CLI or skill | Natural language tools |
| What's stored | Static prompt | Keyword (memory query) |
| At fire time | Replays saved prompt | Recalls current facts |
| Adapts to changes | No (same message) | Yes (queries memory) |
| "Check less often" | Edit config, restart | "Check on me every 4 hours" |
| Token cost | High (full context) | Lower (cron triggers isolated) |
| One-time reminders | Yes | Yes (`one_time: true`) |
| Pause temporarily | Delete and recreate | `pause_cron` tool |

### Example: The Key Difference

**Other Assistants:**
```
March: "Remind me about my meds every day at 8pm"
→ Creates cron with prompt: "Remind user about meds"

April: "Oh, I also need to take it with food now"
→ This info is NOT connected to the cron

8pm every day:
→ Fires: "Reminder about your meds" (same message forever)
```

**Sheldon:**
```
March: "Remind me about my meds every day at 8pm"
→ Saves fact: medication = "blood pressure meds"
→ Creates cron with keyword: "meds"

April: "Oh, I also need to take it with food now"
→ Saves fact: medication_note = "take with food"

8pm that day:
→ Fires → Recalls "meds" → Finds both facts
→ "Time for your blood pressure meds! Remember to take it with food."
```

The cron adapts because it queries memory at fire time, not when created.

### Why This Matters

Most assistant crons are schedulers with a static prompt. The `HEARTBEAT.md` approach is a checklist pattern — scan tasks, report status.

Sheldon's cron is a wake-up trigger with a memory search. The keyword tells Sheldon what to think about, not what to say. This means:

- Updates to related facts automatically appear in reminders
- Context from recent conversations informs the message
- The same keyword can produce different messages based on current state
- No config file editing or restarts needed to change check-in frequency

---

## Memory

### Other Assistants

```
Session starts → Context window fills up → Pruning kicks in → Old messages gone
```

Memory is the conversation history. When it gets too long, older messages are dropped. Some assistants have optional memory plugins for RAG-based retrieval, but they're separate from the core system.

### Sheldon

```
You: "I moved to Portland last month"
Sheldon: [saves fact: user lives in Portland, domain: Place & Environment]

3 months later...
You: "What's the weather like where I live?"
Sheldon: [recalls: user lives in Portland] "Let me check Portland weather..."
```

**Architecture:**
- SQLite + sqlite-vec for vector search
- Graph structure: entities → facts → edges
- 14 life domains (identity, health, relationships, work, finances, etc.)
- Hybrid retrieval: keyword matching + semantic similarity
- Contradiction detection: new facts automatically supersede old ones
- Confidence scoring with decay over time

### Comparison

| Feature | Other Assistants | Sheldon |
|---------|----------|---------|
| Storage | In-memory (+ optional plugins) | SQLite + vectors |
| Structure | Flat messages | Graph (entities, facts, edges) |
| Persistence | Session only (without plugins) | Lifetime |
| Semantic search | Via plugin | Built-in (Ollama embeddings) |
| Contradictions | Manual | Auto-detected and superseded |
| Domains | None | 14 life domains |
| Recall | Scroll up or plugin | `recall_memory` tool |
| Time-based recall | Automatic decay only | Explicit queries |

### Example: Contradiction Handling

**Other Assistants:**
```
March: "I live in NYC"
June: "I moved to LA"
September: "Where do I live?"
→ Both facts in context, model has to figure it out (or old one pruned)
```

**Sheldon:**
```
March: [saves: city = NYC, confidence 0.9]
June: [saves: city = LA, confidence 0.9] → auto-supersedes NYC fact
September: "Where do I live?"
→ [recalls: city = LA] "You live in LA"
```

### Example: Time-Based Recall

**Other Assistants:**
```
You: "What did I tell you last week?"
→ Time decay scoring prioritizes recent memories automatically
→ Can't explicitly query "last week" - system decides what's relevant
→ Old memories naturally drop in ranking
```

Time decay is automatic - you can't ask for memories from a specific period.

**Sheldon:**
```
You: "What did I tell you yesterday?"
Sheldon: [recalls with time_range: "yesterday"]
→ "Yesterday you mentioned finishing the API refactor and wanting to start on the frontend."

You: "What did we discuss on February 20th?"
Sheldon: [recalls with since: "2025-02-20", until: "2025-02-20"]
→ "On February 20th you talked about your new apartment search criteria."

You: "Show me everything from last week"
Sheldon: [recalls with time_range: "last_week", query: "*"]
→ Lists all facts saved during that period
```

Sheldon has a `current_time` tool so it always knows today's date, and `recall_memory` accepts:
- `time_range`: "today", "yesterday", "this_week", "last_week", "this_month"
- `since` / `until`: specific dates like "2025-02-20"

---

## Stateful Workflows

### Other Assistants

```
You: "Help me plan meals for the week"
Assistant: "Here's a meal plan:
  Mon: Pasta
  Tue: Stir fry
  ..."
*saves to MEMORY.md as flat text*

Next day...
You: "I made the pasta, update the plan"
Assistant: *searches MEMORY.md, rewrites entire section*

A week later...
You: "What's my meal plan?"
Assistant: *old plan still there, mixed with new info*
```

Memory is append-only or flat text. Tracking mutable state requires manual management. No way to automatically check in or remind.

### Sheldon

```
You: "Help me plan meals for the week"
Sheldon: [saves note: "meal_plan" with structured JSON]
{
  "week_of": "2025-02-24",
  "meals": [
    {"day": "Mon", "meal": "Pasta", "done": false},
    {"day": "Tue", "meal": "Stir fry", "done": false}
  ]
}

You: "I made the pasta"
Sheldon: [get_note → update done:true → save_note]
"Updated! Pasta marked as done."

You: "Remind me about dinner at 5pm each day"
Sheldon: [creates cron with keyword "meal_plan"]

5pm...
Cron fires → recalls meal_plan note → sees today's meal
Sheldon: "Tonight's dinner: Stir fry. Need any recipe help?"

End of week...
Sheldon: [deletes old note, optionally saves summary to long-term memory]
```

### The Three-Part System

| Component | Purpose | Example |
|-----------|---------|---------|
| **Notes** | Mutable state | Current meal plan, shopping list, weekly goals |
| **Crons** | Time-based triggers | "Remind me at 5pm daily" |
| **Memory** | Long-term facts | "User is vegetarian", "User dislikes cilantro" |

When combined:
1. **Notes** track what changes frequently (this week's plan)
2. **Memory** informs decisions (dietary preferences)
3. **Crons** drive proactive behavior (daily reminders)

### Comparison

| Feature | Other Assistants | Sheldon |
|---------|------------------|---------|
| Mutable state | Flat text rewrite | Key-value notes |
| Structured data | No | JSON in notes |
| Proactive reminders | No crons | Cron + memory query |
| State + schedule | Separate systems | Integrated |
| Delete when done | Manual cleanup | `delete_note` |
| Context efficiency | Full history loaded | Keys only, content on-demand |

### Real Workflows This Enables

**Meal Planning:**
- Track weekly meals with done/not-done status
- Daily dinner reminders that know today's meal
- Grocery list that items get crossed off
- Remembers dietary restrictions from long-term memory

**Habit Tracking:**
- Note tracks streak count and last completion
- Daily cron asks "Did you exercise today?"
- Updates streak on response
- Remembers your exercise preferences

**Project Management:**
- Note tracks current sprint tasks
- Cron checks in every few hours
- Recalls recent facts about blockers
- Cleans up completed items

**Shopping Lists:**
- Note stores current list as array
- "Add milk" → appends to list
- "Got the milk" → removes from list
- "What do I need?" → reads current list

### Why Context Efficiency Matters

**System prompt sees only:**
```
## Active Notes
meal_plan, shopping_list, weekly_goals
```

**Not:**
```
## Active Notes
meal_plan: {"week_of": "2025-02-24", "meals": [{"day": "Mon"... (500 tokens)
shopping_list: ["milk", "eggs", "bread"... (200 tokens)
weekly_goals: {"goal_1": "Finish API"... (300 tokens)
```

Content loads only when Sheldon calls `get_note`. This keeps the base context small while allowing unlimited structured state.

---

## Code to Deploy

### Other Assistants

```
You: "Build me a weather dashboard"
Assistant: [writes code to local files]
Assistant: "I've created the files. Run `npm start` to test it."

You: *opens terminal, runs npm start, tests locally*
You: *figures out deployment yourself*
```

Most assistants write code. Deployment is your problem.

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

| Step | Other Assistants | Sheldon |
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

### Other Assistants

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

| Layer | Other Assistants | Sheldon |
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

### Other Assistants

```
You: "Save this document"
Assistant: [writes to local filesystem]
Assistant: "Saved to ~/documents/note.md"

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

| Feature | Other Assistants | Sheldon |
|---------|----------|---------|
| Storage | Local filesystem | MinIO (S3-compatible) |
| Share files | Manual | `share_link` (presigned URLs) |
| Fetch from URL | No | `fetch_url` (up to 100MB) |
| Organized buckets | No | user, agent, backups |
| Memory backup | No | `backup_memory` |
| Access from anywhere | No | Yes (MinIO has web UI) |

---

## Homelab & Networking

### Other Assistants

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

| Feature | Other Assistants | Sheldon |
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

### Other Assistants

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

| Feature | Other Assistants | Sheldon |
|---------|----------|---------|
| Format | JSON + JS/TS | Markdown |
| Source | Central marketplace | Any URL |
| Install | `/skill install x` | "Install this URL" |
| Verification | Trust the marketplace | Read before install |
| Discovery | Marketplace search | Web search |
| Create | Publish to registry | Just write markdown |

### The Difference

Some assistants have bigger ecosystems but you're trusting third-party code.

Sheldon has fewer skills but you verify each one. You can read the markdown, understand what it does, then install. Or ask Sheldon to browse the web for skills and review them together.

---

## Vision & Media

### Other Assistants

```
# Supports vision models
/model gpt-4o
*send image*
Assistant: "I can see a cat in this image"

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

| Feature | Other Assistants | Sheldon |
|---------|----------|---------|
| Receive images | Yes | Yes |
| Analyze images | Yes | Yes |
| Video analysis | Yes (Gemini, separate subsystem) | Yes (Claude, inline) |
| Save to storage | No | MinIO upload |
| Send images back | No | `send_image` |
| Capability detection | Manual config | Auto-detected per model |

### The Difference

Both handle images and video. Some assistants route video to a separate media-understanding subsystem. Sheldon processes video inline in the conversation using Claude's native video support.

Sheldon also auto-detects capabilities per model:

```
Claude Opus → vision: true, video: true
GPT-4o → vision: true, video: false
Kimi → vision: false
Ollama llava → vision: true
```

If you send an image to a non-vision model, Sheldon tells you instead of failing silently.
