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
1. ~~Domain router - smarter recall~~ (replaced by tool calling)
2. ~~Test semantic search end-to-end~~ ✓
3. ~~k8s manifests for ollama deployment~~ ✓
4. Fact deduplication

---

## 2026-02-19: Tool Calling + Graph Traversal + Decay

### What was built

- **Tool calling infrastructure** (`internal/llm/`, `internal/tools/`)
  - Added `ChatWithTools()` to LLM interface
  - Implemented for both Claude and OpenAI-compatible providers
  - `tools.Registry` for registering and executing tools
  - `recall_memory` tool - LLM decides when to search memory

- **Agent loop refactor** (`internal/agent/agent.go`)
  - `runAgentLoop()` iterates until LLM stops calling tools
  - Max 5 iterations to prevent infinite loops
  - Session tracks tool calls and results

- **Entity graph traversal** (`pkg/koramem/`)
  - `SearchEntities(query)` - fuzzy search entities by name
  - `Traverse(entityID, maxDepth)` - walk graph, collect connected entities + facts
  - `Recall(ctx, query, domains, limit)` - combines hybrid search + traversal

- **Recency weighting** (`pkg/koramem/queries.go`)
  - Read-time decay: older/unaccessed facts rank lower
  - Formula: `score = (confidence * 0.7) + (recency * 0.3)`

- **Memory decay** (`pkg/koramem/decay.go`)
  - `DecayConfig` struct - configurable thresholds
  - `DefaultDecayConfig` - 6 months, access ≤1, confidence ≤0.5
  - `Decay(cfg)` - hard deletes stale facts + embeddings
  - Domain-specific overrides supported for open source flexibility
  - Go ticker in main.go runs decay daily

- **Kubernetes deployment** (`deploy/k8s/`)
  - Full manifests: namespace, ollama, kora, config, secrets, essence
  - Ollama with initContainer to pull nomic-embed-text model
  - PVCs for persistence
  - Tested locally with Docker Desktop Kubernetes

- **Project structure cleanup**
  - Removed unused `router/` package (tool calling replaced it)
  - Moved Dockerfile into `core/` for multi-app monorepo pattern

### How tool calling works
```
User: "What's Sarah's birthday?"
         ↓
Agent: llm.ChatWithTools(messages, [recall_memory])
         ↓
LLM returns: tool_use("recall_memory", {query: "Sarah birthday"})
         ↓
Agent: tools.Execute("recall_memory", args)
         ├── memory.Recall() → hybrid search + entity traversal
         └── Returns facts + related entities
         ↓
Agent: llm.ChatWithTools(messages + tool_result, [recall_memory])
         ↓
LLM returns: "Sarah's birthday is March 15th"
```

### Tested
- Ollama deployed to local k8s ✓
- Embeddings API working ✓
- Graph traversal tests pass ✓
- Decay tests pass ✓

### What's next
1. ~~Deploy full stack (Kora + Ollama) to k8s~~ ✓
2. ~~Test tool calling end-to-end via Telegram~~ ✓
3. ~~Fact deduplication~~ ✓
4. Skills framework (user-invoked commands)

---

## 2026-02-19: Full Stack + Proactive Messaging

### What was built

- **Fact deduplication** (`pkg/koramem/facts.go`)
  - Semantic dedup at write-time using embeddings
  - Similarity threshold: 0.15 cosine distance
  - If similar fact exists: touch `last_accessed` or supersede if newer
  - Prevents duplicate facts from accumulating

- **Full stack K8s deployment**
  - Kora + Ollama running on local Docker Desktop Kubernetes
  - Tool calling verified end-to-end via Telegram
  - Fixed image tag mismatch (`kora:local` → `kora:latest`)
  - Go version fix in Dockerfile (1.22 → 1.24)

- **Heartbeat system** (`internal/agent/heartbeat.go`)
  - Proactive check-ins based on stored context
  - Recalls goals (D10), routines (D12), events (D13)
  - LLM crafts contextual message (1-2 sentences)
  - Fires immediately on startup (10s delay), then every N hours
  - Config: `HEARTBEAT_ENABLED`, `HEARTBEAT_INTERVAL`, `HEARTBEAT_CHAT_ID`

- **Discord support** (`internal/bot/discord.go`)
  - Full implementation using discordgo
  - Same interface as Telegram: `Start()`, `Send()`
  - Session IDs: `discord:<channel_id>`

- **Multi-provider bots** (`internal/bot/`, `internal/config/`)
  - Both Telegram and Discord can run simultaneously
  - Auto-detected by token presence (`TELEGRAM_TOKEN`, `DISCORD_TOKEN`)
  - Shared memory across platforms
  - Separate sessions per channel

- **Logging improvements**
  - Added session ID to telegram message log
  - Proactive message logging in `Send()` methods

### How heartbeat works
```
Startup
   ↓
go func() { time.Sleep(10s); sendHeartbeat(); time.Tick(interval)... }
   ↓
Heartbeat()
   ├── memory.Recall("goals tasks events", [10,12,13], 10)
   ├── llm.Chat() with context → "Hey, how's the project going?"
   └── session.AddMessage() for conversation continuity
   ↓
bot.Send(chatID, message)
```

### How multi-provider works
```
main.go
   ├── if TELEGRAM_TOKEN → NewTelegram() → go bot.Start()
   ├── if DISCORD_TOKEN → NewDiscord() → go bot.Start()
   └── heartbeat uses bots[0] (first enabled)

All bots share:
   - Same agent loop
   - Same koramem (long-term memory)
   - Different sessions (telegram:123 vs discord:456)
```

### Tested
- Semantic dedup verified via access_count increments ✓
- Tool calling end-to-end via Telegram ✓
- Heartbeat message received on Telegram ✓
- Multi-provider startup with `bots=[telegram]` ✓

### What's next
1. Deploy to Hetzner (production)
2. Add Discord token for multi-platform
3. Phase 2 features (voice, deeper integrations)

---
