# SESSION.md — Development Journal

> Quick notes after each session. What was built, how it connects, what's next.

---

## 2026-02-17: Initial Setup

### What was built
- **kora-core**: Minimal Telegram bot with Claude
  - `bot/telegram.go` receives messages, calls agent
  - `agent/agent.go` manages sessions, calls LLM
  - `session/session.go` stores conversation history in-memory
  - `llm/claude.go` wraps Anthropic SDK
  - `config/config.go` loads from env vars
  - System prompt loaded from SOUL.md

- **pkg/koramem**: Memory package (not wired yet)
  - SQLite with WAL mode
  - Schema: domains, entities, facts, edges
  - 14 domains seeded on init
  - Kora agent entity seeded on init
  - Contradiction detection in AddFact()

### How it connects
```
Telegram → bot.handleMessage() → agent.Process() → session.Get() + llm.Chat() → response
```

koramem is standalone, not connected to agent yet.

### What's next
1. Run the bot, verify it works
2. Write koramem tests to understand it
3. Wire koramem into agent:
   - On message: Recall relevant facts
   - After response: Remember new facts
4. Build domain router

---

## 2026-02-18: Memory Loop Complete

### What was built

- **LLM abstraction** (`internal/llm/`)
  - Interface-based: `llm.LLM` with `Chat()` method
  - Providers: Claude, OpenAI, Kimi K2
  - Config-driven model selection
  - Separate extractor LLM for cheap fact extraction

- **Bot abstraction** (`internal/bot/`)
  - Interface-based: `bot.Bot` with `Start()` method
  - Providers: Telegram (Discord stub)
  - Platform-prefixed session IDs: `telegram:123456`

- **Memory integration** (`internal/agent/`)
  - Recall: searches facts across all domains before LLM call
  - Remember: extracts facts from conversation using Kimi K2
  - User entities created automatically: `user_telegram_<id>`

- **Logging** (`internal/logger/`)
  - Structured logging with `log/slog`
  - Levels: Debug, Info, Warn, Error, Fatal
  - `KORA_DEBUG=true` for verbose output

- **Config** (`internal/config/`)
  - `.env` file support via godotenv
  - Separate LLM and Extractor configs
  - Dev defaults: Kimi K2 for everything (cheap)

- **Essence folder refactor**
  - Renamed from `workspace` to `essence`
  - `SOUL.md` - personality (committed)
  - `IDENTITY.md` - personal data (gitignored)
  - `IDENTITY.md.example` - template (committed)

### How it connects
```
Telegram
   ↓
bot.handleMessage()
   ↓
agent.Process()
   ├── recall(message) → search koramem → inject facts into prompt
   ├── llm.Chat() → get response
   └── go remember() → extract facts with Kimi K2 → store in koramem
   ↓
response
```

### Tested
- Bot responds via Telegram ✓
- Facts extracted and stored ✓
- User entity created ✓
- Multiple domains populated (routines, health, career, goals) ✓

### What's next
1. Domain router - smarter recall (not all 14 domains)
2. Fact deduplication - avoid storing duplicates
3. Session persistence - survive restarts
4. IDENTITY.md seeding - bootstrap koramem from file

---

## 2026-02-19: Vector Search Infrastructure Complete

### What was built

- **sqlite-vec integration** for semantic search
  - CGo-free WASM approach using `ncruces/go-sqlite3`
  - Working version combination found:
    - `sqlite-vec-go-bindings v0.0.1-alpha.37`
    - `ncruces/go-sqlite3 v0.17.2-0.20240711235451-21de85e849b7` (specific commit)
    - `wazero v1.7.3`
  - Note: Newer tagged releases (v0.1.6) have broken WASM binaries

- **Vector operations** (`pkg/koramem/vectors.go`)
  - `Embedder` interface for pluggable embedding providers
  - `EmbedFact()` - store embedding when fact is created
  - `SemanticSearch()` - KNN search on vec_facts table
  - `HybridSearch()` - combines keyword + semantic results
  - `ReindexEmbeddings()` - regenerate all embeddings

- **Schema** (`pkg/koramem/schema.go`)
  - `vec_facts` virtual table using vec0
  - 1536 dimensions (OpenAI/Voyage compatible)

### How it connects
```
AddFact()
   ├── Insert into facts table
   └── EmbedFact() → embedder.Embed() → insert into vec_facts

HybridSearch(query)
   ├── SearchFacts() → keyword LIKE search
   ├── SemanticSearch() → embedder.Embed(query) → KNN on vec_facts
   └── Merge results (semantic first, then keyword)
```

### Debugging notes
- Tagged releases (v0.1.6, v0.1.7-alpha) of sqlite-vec-go-bindings have WASM built against wrong ncruces API
- Official example in sqlite-vec repo uses alpha.37 + specific commit
- Without threads enabled: "atomic store disabled" error
- With threads but wrong versions: "go_busy_timeout signature mismatch"

### What's next
1. ~~Implement Embedder~~ ✓
2. ~~Wire embedder into agent~~ ✓
3. Domain router - smarter recall
4. Test semantic search end-to-end (needs Ollama running)

---

## 2026-02-19: Embedder Implementation

### What was built

- **Embedder package** (`internal/embedder/`)
  - `Embedder` interface (defined in koramem)
  - `ollama.go` - HTTP client for Ollama API
  - Config-driven provider selection

- **Config updates** (`internal/config/`)
  - Added `EmbedderConfig` with Provider, BaseURL, Model
  - Env vars: `EMBEDDER_PROVIDER`, `EMBEDDER_URL`, `EMBEDDER_MODEL`

- **Main wiring** (`cmd/kora/main.go`)
  - Creates embedder from config
  - Sets embedder on memory store via `memory.SetEmbedder(emb)`
  - Logs embedder provider on startup

### Architecture
```
Kubernetes cluster:
├── kora (deployment)
│   ├── single binary with koramem embedded
│   ├── calls ollama via HTTP for embeddings
│   └── sqlite-vec for vector storage
└── ollama (deployment)
    └── nomic-embed-text model (768 dims, 274MB)
```

### Environment variables
```bash
EMBEDDER_PROVIDER=ollama
EMBEDDER_URL=http://ollama:11434  # k8s service name
EMBEDDER_MODEL=nomic-embed-text
```

### What's next
1. Domain router - smarter recall (not all 14 domains)
2. Test semantic search end-to-end
3. k8s manifests for ollama deployment
4. Fact deduplication

---
