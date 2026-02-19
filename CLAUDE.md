# CLAUDE.md — Kora Project Guide

> Kora is a personal AI assistant that knows your entire life across 14 structured domains, running on your own infrastructure.

## Current Architecture

```
┌─────────────────────────────────────────────────────────┐
│                         KORA                            │
│                                                         │
│   Telegram ──► bot/telegram.go                          │
│       │                                                 │
│       ▼                                                 │
│   agent/agent.go ──► session/session.go (in-memory)     │
│       │                                                 │
│       ├──► SOUL.md (system prompt)                      │
│       │                                                 │
│       ▼                                                 │
│   llm/claude.go ──► Anthropic API                       │
│                                                         │
│   ┌─────────────────────────────────────────────────┐   │
│   │  pkg/koramem (NOT WIRED YET)                    │   │
│   │  SQLite: domains, entities, facts, edges        │   │
│   └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## File Structure

```
kora/
├── CLAUDE.md              # This file - project instructions
├── SOUL.md                # Kora's personality (system prompt)
├── IDENTITY.md            # Bootstrap user facts
├── DECISIONS.md           # Architecture decisions log
├── SESSION.md             # Dev session journal
│
├── kora-core/             # Main application
│   ├── cmd/kora/main.go
│   └── internal/
│       ├── agent/         # Agent loop, processes messages
│       │   ├── types.go
│       │   └── agent.go
│       ├── bot/           # Telegram integration
│       │   ├── types.go
│       │   └── telegram.go
│       ├── config/        # Environment config
│       │   ├── types.go
│       │   └── config.go
│       ├── llm/           # Claude API client
│       │   ├── types.go
│       │   └── claude.go
│       ├── router/        # Domain classification (stub)
│       │   ├── types.go
│       │   └── router.go
│       └── session/       # In-memory conversation state
│           ├── types.go
│           └── session.go
│
└── pkg/koramem/           # Memory package (standalone)
    ├── types.go           # Store, Domain, Entity, Fact, Edge
    ├── schema.go          # SQLite DDL
    ├── store.go           # Open, Close, migrate
    ├── domains.go         # 14 life domains
    ├── entities.go        # Entity CRUD
    ├── facts.go           # Fact CRUD + contradiction detection
    └── edges.go           # Edge CRUD
```

## Development Workflow

### 1. Build Vertically

Don't build entire packages before wiring them. Build thin slices end-to-end:

- Bad: Build all of koramem → then wire to agent → then test
- Good: User says "my name is X" → agent extracts → koramem stores → next message recalls

### 2. Run Early, Run Often

```bash
# Set env vars
export TELEGRAM_TOKEN="your-token"
export ANTHROPIC_API_KEY="your-key"
export KORA_WORKSPACE="/path/to/kora"

# Run
cd kora-core && go run ./cmd/kora
```

### 3. Tests as Documentation

Write tests to understand code. Tests show how pieces connect:

```bash
cd pkg/koramem && go test -v
```

### 4. Session Journal

After each session, update SESSION.md:

- What was built
- How it connects
- What's next

## Code Style

- No unnecessary comments. Code should be self-documenting.
- No emojis in code or comments.
- Each package has `types.go` for structs, main logic in other files.
- Small, focused functions.
- Actionable error messages.
- Run `go fmt` and `go vet` before committing.

## The 14 Domains

| ID  | Name                    | Layer    |
| --- | ----------------------- | -------- |
| 1   | Identity & Self         | Core     |
| 2   | Body & Health           | Core     |
| 3   | Mind & Emotions         | Inner    |
| 4   | Beliefs & Worldview     | Inner    |
| 5   | Knowledge & Skills      | Inner    |
| 6   | Relationships & Social  | World    |
| 7   | Work & Career           | World    |
| 8   | Finances & Assets       | World    |
| 9   | Place & Environment     | World    |
| 10  | Goals & Aspirations     | Temporal |
| 11  | Preferences & Tastes    | Meta     |
| 12  | Rhythms & Routines      | Temporal |
| 13  | Life Events & Decisions | Temporal |
| 14  | Unconscious Patterns    | Meta     |

## Quick Reference

```bash
# Build
cd kora-core && go build -o bin/kora ./cmd/kora

# Test koramem
cd pkg/koramem && go test -v

# Run
cd kora-core && ./bin/kora
```
