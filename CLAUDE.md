# CLAUDE.md — Sheldon Project Guide

> Sheldon is a personal AI assistant that remembers your entire life across 14 structured domains, runs on your own infrastructure, and can write and deploy code autonomously.

## Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                              SHELDON                                 │
│                                                                      │
│  Telegram/Discord ──► Agent Loop ──► LLM (Kimi/Claude/OpenAI)        │
│         │                 │                                          │
│         │                 ├── SOUL.md (personality)                  │
│         │                 ├── IDENTITY.md (user bootstrap)           │
│         │                 └── Tools                                  │
│         │                       ├── recall_memory / save_memory      │
│         │                       ├── write_code (→ Coder Sandbox)     │
│         │                       ├── deploy_app (→ Docker Compose)    │
│         │                       ├── set_cron / list_crons            │
│         │                       ├── browse / search_web (→ Browser)  │
│         │                       ├── switch_model / list_models       │
│         │                       ├── pull_model / remove_model        │
│         │                       └── save_skill / list_skills         │
│         │                                                            │
│         └──► sheldonmem (SQLite + sqlite-vec)                        │
│                   ├── Entities (graph nodes)                         │
│                   ├── Facts (domain-tagged, confidence-scored)        │
│                   ├── Edges (typed relationships)                    │
│                   └── Vectors (semantic search via Ollama)           │
│                                                                      │
│  Ollama (sidecar)                                                    │
│    ├── nomic-embed-text (embeddings)                                 │
│    └── qwen2.5:3b (fact extraction)                                  │
│                                                                      │
│  Coder Sandbox (ephemeral containers)                                │
│    └── Claude Code CLI + Kimi K2.5                                   │
│                                                                      │
│  Browser Sandbox (ephemeral containers)                              │
│    └── agent-browser + Chromium                                      │
└──────────────────────────────────────────────────────────────────────┘
```

## File Structure

```
sheldon/
├── CLAUDE.md              # this file
├── README.md              # public docs
├── DECISIONS.md           # architecture decision log
│
├── core/                  # main Go application
│   ├── cmd/sheldon/       # entry point
│   ├── essence/           # SOUL.md, IDENTITY.md
│   ├── deploy/            # docker-compose, coder-sandbox Dockerfile
│   └── internal/
│       ├── agent/         # agent loop, context builder, cron runner
│       ├── bot/           # telegram, discord
│       ├── browser/       # sandboxed browser automation
│       ├── coder/         # code generation bridge, git ops
│       ├── config/        # env config, runtime config
│       ├── conversation/  # recent message buffer (FIFO)
│       ├── cron/          # cron storage
│       ├── deployer/      # docker compose deployer
│       ├── embedder/      # ollama embeddings
│       ├── llm/           # multi-provider (kimi, claude, openai, ollama)
│       ├── storage/       # minio client
│       └── tools/         # all agent tools
│
├── pkg/sheldonmem/        # memory package (standalone, extractable)
│   ├── store.go           # Open, Close, DB
│   ├── entities.go        # entity CRUD
│   ├── facts.go           # fact CRUD, contradiction detection
│   ├── edges.go           # relationship edges
│   ├── vectors.go         # sqlite-vec integration
│   ├── recall.go          # hybrid retrieval (keyword + semantic)
│   ├── decay.go           # memory decay/cleanup
│   └── domains.go         # 14 life domains
│
├── skills/                # markdown skill definitions
└── docs/                  # deployment guide, voice architecture
```

## Key Patterns

### LLM Providers

```go
// internal/llm/llm.go - factory pattern
model, _ := llm.New(llm.Config{
    Provider: "kimi",  // or claude, openai, ollama
    APIKey:   key,
    Model:    "kimi-k2-0711-preview",
})
```

### Tool Registration

```go
// internal/tools/registry.go
tools.RegisterCronTools(agentLoop.Registry(), cronStore, timezone)
tools.RegisterCoderTool(agentLoop.Registry(), bridge, memory)
tools.RegisterUnifiedBrowserTools(agentLoop.Registry(), browserRunner, config)
```

### Memory Operations

```go
// sheldonmem - fact storage with contradiction detection
fact, _ := memory.AddFact(&entityID, domainID, "city", "Berlin", 0.9)
// automatically supersedes previous "city" fact for same entity

// hybrid recall (keyword + semantic)
result, _ := memory.Recall(ctx, "user's location", []int{9}, 5)
```

### Coder Security Model

```
Sheldon (has GIT_TOKEN)
  │
  ├── GitOps.CloneRepo() ── clones repo to workspace
  │
  ├── Spawn coder container (NO GIT_TOKEN)
  │   └── Coder writes code, can't access token
  │
  └── GitOps.PushChanges() ── pushes results to branch
```

## Development

```bash
# local run
cp core/.env.example core/.env
# fill in TELEGRAM_TOKEN, KIMI_API_KEY
cd core && go run ./cmd/sheldon

# test memory package
cd pkg/sheldonmem && go test -v

# build
cd core && go build -o bin/sheldon ./cmd/sheldon
```

## Deployment

Push to main triggers GitHub Actions:

1. Build + push Docker images to GHCR
2. Fetch secrets from Doppler
3. SSH to VPS, deploy via docker-compose

Required Doppler secrets: `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY`, `GHCR_TOKEN`, `TELEGRAM_TOKEN`, `KIMI_API_KEY`

## The 14 Domains

| ID  | Domain                  | Layer    | Rate of Change |
| --- | ----------------------- | -------- | -------------- |
| 1   | Identity & Self         | Core     | Years          |
| 2   | Body & Health           | Core     | Months         |
| 3   | Mind & Emotions         | Inner    | Months         |
| 4   | Beliefs & Worldview     | Inner    | Years          |
| 5   | Knowledge & Skills      | Inner    | Months         |
| 6   | Relationships & Social  | World    | Months         |
| 7   | Work & Career           | World    | Months         |
| 8   | Finances & Assets       | World    | Days           |
| 9   | Place & Environment     | World    | Months         |
| 10  | Goals & Aspirations     | Temporal | Weeks          |
| 11  | Preferences & Tastes    | Meta     | Years          |
| 12  | Rhythms & Routines      | Temporal | Weeks          |
| 13  | Life Events & Decisions | Temporal | Append-only    |
| 14  | Unconscious Patterns    | Meta     | Years          |

## Code Style

- no unnecessary comments
- no emojis in code
- `types.go` for structs in each package
- small focused functions
- `go fmt` and `go vet` before commit
