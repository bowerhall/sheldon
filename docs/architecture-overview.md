# Architecture Overview

## System Architecture (v5)

Sheldon is a single Go binary running on Docker Compose. The memory system (sheldonmem) is an embedded Go package — no external services for memory operations.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER INTERACTION                               │
│                                                                             │
│                    Telegram ◄───────────────► Discord                       │
│                         │                         │                         │
│                         └──────────┬──────────────┘                         │
│                                    │ long-polling (no inbound ports)        │
│                                    ▼                                        │
└────────────────────────────────────┼────────────────────────────────────────┘
                                     │
┌────────────────────────────────────┼─────────────────────────────────────────┐
│                              SHELDON CORE                                    │
│                                    │                                         │
│                                    ▼                                         │
│  ┌───────────────────────────────────────────────────────────────────────┐   │
│  │                           AGENT LOOP                                  │   │
│  │                                                                       │   │
│  │   User Message ──► Context Builder ──► LLM Call ──► Tool Execution    │   │
│  │                          │                              │             │   │
│  │                          │                              ▼             │   │
│  │              ┌───────────┴───────────┐          ┌─────────────┐       │   │
│  │              │                       │          │   TOOLS     │       │   │
│  │              ▼                       ▼          │             │       │   │
│  │         SOUL.md               Session History   │ • recall    │       │   │
│  │      (personality)            (recent context)  │ • remember  │       │   │
│  │                                                 │ • write_code│       │   │
│  │                                                 │ • deploy    │       │   │
│  │                                                 │ • browse    │       │   │
│  │                                                 │ • storage   │       │   │
│  │                                                 └──────┬──────┘       │   │
│  └─────────────────────────────────────────────────────────┼─────────────┘   │
│                                                            │                 │
└────────────────────────────────────────────────────────────┼─────────────────┘
                                                             │
                    ┌────────────────────────────────────────┼────────────────┐
                    │                                        │                │
                    ▼                                        ▼                ▼
┌───────────────────────────┐  ┌─────────────────────────────────┐  ┌─────────────┐
│       SHELDONMEM          │  │         CODE GENERATION         │  │   STORAGE   │
│    (SQLite + sqlite-vec)  │  │                                 │  │   (MinIO)   │
│                           │  │  ┌───────────────────────────┐  │  │             │
│  ┌─────────┐ ┌─────────┐  │  │  │     Ollama + Kimi K2.5    │  │  │  • uploads  │
│  │Entities │ │  Facts  │  │  │  │  (subprocess or Docker)   │  │  │  • backups  │
│  │ (graph) │ │(14 doms)│  │  │  └─────────────┬─────────────┘  │  │  • files     │
│  └────┬────┘ └────┬────┘  │  │                │                │  └─────────────┘
│       │           │       │  │                ▼                │
│  ┌────┴───────────┴────┐  │  │  ┌───────────────────────────┐  │
│  │       Edges         │  │  │  │    Model Selection        │  │
│  │   (relationships)   │  │  │  │                           │  │
│  └─────────────────────┘  │  │  │  NVIDIA_API_KEY set?      │  │
│                           │  │  │         │                 │  │
│  ┌─────────────────────┐  │  │  │    yes  │  no             │  │
│  │      Vectors        │  │  │  │         ▼                 │  │
│  │  (semantic search)  │  │  │  │  ┌──────┴───────┐         │  │
│  └─────────────────────┘  │  │  │  ▼              ▼         │  │
│                           │  │  │ NVIDIA NIM    Kimi API    │  │
│  Single file: sheldon.db   │  │  │ (free tier)   (fallback)  │  │
└───────────────────────────┘  │  │     │              │      │  │
                               │  │     └──────┬───────┘      │  │
                               │  │            ▼              │  │
                               │  │     kimi-k2.5 model       │  │
                               │  │            │              │  │
                               │  │            ▼              │  │
                               │  │   Code / Files / Git Push │  │
                               │  └───────────────────────────┘  │
                               └─────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                            LLM PROVIDERS                                    │
│                                                                             │
│   Main Chat & Memory Extraction          Code Generation                    │
│   ┌─────────────────────────┐            ┌─────────────────────────┐        │
│   │  Kimi (kimi-k2-0711)    │            │  Ollama + Kimi K2.5     │        │
│   │  via KIMI_API_KEY       │            │  via NVIDIA or Kimi API │        │
│   └─────────────────────────┘            └─────────────────────────┘        │
│                                                                             │
│   Supported: claude, openai, kimi        Runs in isolated sandbox/Docker    │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                          INFRASTRUCTURE (VPS)                               │
│                                                                             │
│   Hetzner CX32 (8GB RAM, €8/mo) running Docker Compose                      │
│                                                                             │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                         │
│   │   Sheldon   │  │   Traefik    │  │  Deployed   │                         │
│   │  Container  │  │   (proxy)   │  │    Apps     │                         │
│   │             │  │             │  │             │                         │
│   │  main app   │  │   routing   │  │  user apps  │                         │
│   └─────────────┘  └─────────────┘  └─────────────┘                         │
│          │                │                │                                │
│          └────────────────┴────────────────┘                                │
│                           │                                                 │
│                    Docker Volumes                                           │
│                    ./data, ./apps.yml                                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Message Flow

```
1. User sends Telegram/Discord message
2. Bot channel receives message (long-polling, no inbound ports)
3. Agent loop starts processing
4. Domain Router → LLM call → returns domain IDs
5. sheldonmem.Recall() → hybrid search (keyword + semantic + graph expansion)
6. ContextBuilder → assembles: SOUL.md + entity facts + session history
7. LLM API call → response
8. If tool calls → execute tools → loop back to step 7
9. Final response sent to user
10. sheldonmem.Remember() → async: extract facts, store to entities
```

## Code Generation Flow

```
1. User requests code task via chat
2. Agent calls write_code tool with prompt
3. Bridge creates isolated workspace (sandbox or Docker container)
4. Ollama launches with kimi-k2.5 model
   └── Model selection: NVIDIA NIM (free) → Kimi API (fallback)
5. Coder writes/edits files in workspace
6. Output sanitized (API keys, tokens stripped)
7. Files collected, workspace path returned
8. Optional: build_image → deploy via compose
9. Optional: git commit/push to configured repo
```

## The 14 Life Domains

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

## Deployment Modes

| Mode       | RAM | What's Included                    |
| ---------- | --- | ---------------------------------- |
| `minimal`  | 2GB | Sheldon only, no web routing       |
| `standard` | 4GB | Sheldon + Traefik + app deployment |

## Cost Breakdown

| Component           | Cost        |
| ------------------- | ----------- |
| Hetzner CX32 VPS    | €8/mo       |
| NVIDIA NIM API      | Free        |
| Kimi API (fallback) | Pay-per-use |
| **Total**           | ~€8/mo      |

## Why sheldonmem Over Markdown Memory

Basic assistants store memory in markdown files. This works for simple use cases but lacks:

- Structured domains
- Semantic search
- Entity relationships
- Contradiction detection
- Decay scoring

sheldonmem replaces this with:

- SQLite + sqlite-vec in a single file
- 14 life domains as first-class schema
- Entity graph with typed relationships
- Hybrid retrieval (keyword + semantic + graph)
- Automatic memory hygiene

Still lightweight (~100–200MB RAM), still a single binary, but with real memory capabilities.
