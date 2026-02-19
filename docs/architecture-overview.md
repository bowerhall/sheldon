# Architecture Overview

## System Architecture (v5)

Kora is a single Go binary running on k3s. The memory system (koramem) is an embedded Go package — no external services for memory operations.

```
┌─────────────────────────────────────────────────────────────────────┐
│                        KORA SYSTEM (k3s cluster)                    │
│                                                                     │
│  ┌────────────────────── kora-system namespace ──────────────────┐  │
│  │                                                               │  │
│  │  ┌─────────────────────────────────────────────────────────┐  │  │
│  │  │              KORA (PicoClaw Fork + koramem)             │  │  │
│  │  │                                                         │  │  │
│  │  │  Telegram ──→ Agent Loop ──→ ContextBuilder             │  │  │
│  │  │  Channel       │    ↑         │                         │  │  │
│  │  │                │    │         ├── SOUL.md               │  │  │
│  │  │                │    │         ├── IDENTITY.md           │  │  │
│  │  │                │    │         ├── session history       │  │  │
│  │  │         ┌──────┘    │         └── koramem.Recall()      │  │  │
│  │  │         ↓           │              (in-process)         │  │  │
│  │  │  Domain Router ─────┤                                   │  │  │
│  │  │  (Haiku call)       │         koramem.Remember()        │  │  │
│  │  │         │           │         (post-response, async)    │  │  │
│  │  │         ↓           ↓                                   │  │  │
│  │  │  ┌──────────────────────────────────────────┐           │  │  │
│  │  │  │  koramem (embedded Go package)           │           │  │  │
│  │  │  │  SQLite + sqlite-vec — single kora.db    │           │  │  │
│  │  │  │  entities, facts, edges, domains, vectors │          │  │  │
│  │  │  └──────────────────────────────────────────┘           │  │  │
│  │  │                                                         │  │  │
│  │  │  Tool Registry    Session Manager    Cron/Heartbeat     │  │  │
│  │  └─────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌────────────────────── kora-storage namespace (Phase 3+) ──────┐  │
│  │  MinIO (object storage) — backups, artifacts, exports         │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌────────────────────── future namespaces ──────────────────────┐  │
│  │  kora-voice:    Piper/XTTS (Phase 4+)                         │  │
│  │  kora-services: Agent-deployed apps (Phase 6+)                │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘

External:
  Telegram API ←→ Kora (outbound HTTPS)
  Claude API   ←→ Kora (outbound HTTPS, for responses + extraction + routing)
  Embed API    ←→ Kora (outbound HTTPS, Voyage AI or local Ollama)
```

## Message Flow

```
1. User sends Telegram message
2. PicoClaw Telegram channel receives message
3. Agent loop starts processing
4. Domain Router → Haiku call → returns domain IDs + model tier
5. koramem.Recall() → hybrid search (keyword + semantic + graph expansion)
5b. Agent self-load → always fetch Kora entity facts (nickname, tone, corrections)
6. ContextBuilder → assembles: SOUL.md + Kora entity facts + user facts + session history
7. Claude API call (model selected by router) → response
8. If tool calls → execute tools → loop back to step 7
9. Final response sent to user via Telegram
10. koramem.Remember() → async: extract user-directed + agent-directed facts, store to respective entities
```

## Phase Evolution

| Phase | What's Added                                                        | Containers   |
| ----- | ------------------------------------------------------------------- | ------------ |
| 0     | Kora binary with koramem                                            | 1            |
| 1     | Structured logging, budget tracking, briefings                       | 1            |
| 2     | Claude Code bridge ([spec](claude-code-bridge.md)), skill execution | 1            |
| 3     | MinIO storage, backup automation                                    | 2            |
| 4     | Voice server (Piper/XTTS)                                           | 3            |
| 5     | Mac titlebar app                                                    | 3 + desktop  |
| 6     | Self-extension engine                                               | 3+ (dynamic) |
| 7     | Mobile apps                                                         | 3+ (dynamic) |

## Why koramem Over PicoClaw's Markdown Memory

PicoClaw stores memory in markdown files (MEMORY.md, USER.md, sessions/). This works for simple assistants but lacks: structured domains, semantic search, entity relationships, contradiction detection, and decay scoring.

koramem replaces this with: SQLite + sqlite-vec in a single file, 14 life domains as first-class schema, entity graph with typed relationships, hybrid retrieval (keyword + semantic + graph), and automatic memory hygiene. Still lightweight (~100–200MB RAM), still a single binary, but with real memory capabilities.
