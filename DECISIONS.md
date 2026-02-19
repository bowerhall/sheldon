# DECISIONS.md — Development Decision Log

> On-the-fly decisions and tradeoffs made during development. Newest first.

---

## 2026-02-19: Single Heartbeat System

### One proactive system instead of multiple crons
**Decision**: Use a single heartbeat system that adapts based on context, rather than separate crons for morning briefing, goal tracking, etc.

**Why**: The LLM + koramem can determine what's relevant. If heartbeat fires in the morning, it naturally includes briefing-style content. If it knows sleep patterns (from conversations), it times messages appropriately. One system, contextually aware.

**Tradeoff**: Less predictable than hardcoded "8am briefing". Acceptable because the goal is natural conversation, not rigid schedules.

---

## 2026-02-19: Immediate Heartbeat on Startup

### Fire first heartbeat after 10s delay
**Decision**: Heartbeat fires immediately after startup (10s delay for bot init), then continues at configured interval.

**Why**: Useful for testing and makes sense in production — Kora checks in shortly after coming online. The 10s delay ensures the Telegram/Discord bot is fully connected.

**Tradeoff**: Users get a message every time Kora restarts. Acceptable; restarts are rare and the check-in is contextual.

---

## 2026-02-19: Multi-Provider by Token Presence

### Auto-detect enabled bots from environment
**Decision**: If `TELEGRAM_TOKEN` is set, Telegram bot starts. If `DISCORD_TOKEN` is set, Discord bot starts. Both can run simultaneously.

**Why**: Simple configuration, no explicit "enable" flags needed. Users just set the tokens for platforms they want.

**Tradeoff**: Can't disable a platform without removing its token. Fine for our use case.

---

## 2026-02-19: Semantic Deduplication at Write-Time

### Check similarity before inserting new facts
**Decision**: When adding a fact, check if a semantically similar fact already exists (0.15 cosine distance threshold). If found, touch `last_accessed` or supersede.

**Why**: Read-time filters (recency weighting) don't prevent duplicate facts from accumulating. Over time, "user likes coffee" could appear 50 times. Write-time dedup keeps the store clean.

**Tradeoff**: Extra embedding + search on every write. Acceptable cost for data quality. Also catches cross-field duplicates ("favorite drink" vs "morning beverage").

---

## 2026-02-19: Tool Calling over Domain Router

### LLM-driven recall instead of routing layer
**Decision**: Replaced domain router with tool calling. LLM decides when to search memory via `recall_memory` tool.

**Why**: Domain routing added an extra LLM call to classify which domains to search. Tool calling also uses LLM calls, but is more flexible - the model can search multiple times, refine queries, or skip search entirely when not needed.

**Tradeoff**: LLM might not always call the tool when it should. In practice, models are good at this. If recall is missed, user can rephrase.

---

## 2026-02-19: Configurable Decay for Open Source

### DecayConfig struct with sensible defaults
**Decision**: Made memory decay configurable via `DecayConfig` struct rather than hardcoded values.

**Why**: koramem will be open-sourced. Different users have different needs - some want aggressive decay, some want to keep everything. Provide `DefaultDecayConfig` for simplicity, allow overrides for flexibility.

**Defaults**: 6 months age, access count ≤1, confidence ≤0.5. Domain-specific overrides supported.

---

## 2026-02-19: Dockerfile per App

### Dockerfile inside each app directory
**Decision**: Moved Dockerfile from repo root into `core/`. Build with `docker build -f core/Dockerfile .`

**Why**: Monorepo will have multiple apps (core, voice, mac app). Each app owns its Dockerfile. Build context is always repo root so shared packages (koramem) are accessible.

**Tradeoff**: Slightly longer build command. Worth it for clean multi-app structure.

---

## 2026-02-19: Hard Delete in Decay

### Delete stale facts instead of soft-delete
**Decision**: `Decay()` permanently deletes facts (and their embeddings) rather than setting `active = 0`.

**Why**: Facts meeting decay criteria (old, never accessed, low confidence) are truly disposable. No reason to keep them. Saves storage, keeps search fast.

**Tradeoff**: No recovery. Acceptable because high-confidence or frequently-accessed facts are protected by the criteria.

---

## 2026-02-17: Build from Scratch

### Ditched PicoClaw, built fresh
**Decision**: Removed PicoClaw fork, built kora-core from scratch with clean structure.

**Why**: PicoClaw had unfamiliar naming conventions, flat pkg/ structure, and patterns that didn't match preferred style. Building fresh gives full ownership of ~240 lines vs. adapting 5000+ lines.

**Result**: Clean internal/ structure with bot, agent, llm, config, session, router packages. Binary 9.3MB vs 15MB.

---

## 2026-02-17: Project Setup

### Monorepo structure
**Decision**: Single git repo for all Kora components (kora-core, future kora-mac, kora-mobile).

**Why**: Solo project with tightly coupled components. Atomic commits, shared docs/config, simpler refactoring. No cross-repo dependency management.

**Tradeoff**: Larger repo over time. If koramem needs to be open-sourced separately, extract it later — Go modules make this easy.

---

### PicoClaw as foundation
**Decision**: Fork PicoClaw into `kora-core/` rather than starting from scratch.

**Why**: PicoClaw provides battle-tested Telegram integration, agent loop, session management, LLM provider abstraction, and skill framework. Building these from scratch would add weeks with no differentiation.

**Tradeoff**: We inherit PicoClaw's patterns and must work within its architecture. Acceptable since it's well-designed and lightweight.

---

### koramem replaces markdown memory
**Decision**: Build koramem (SQLite + sqlite-vec) to replace PicoClaw's MEMORY.md/USER.md approach.

**Why**: Markdown files lack semantic search, structured domains, entity relationships, contradiction detection. These are core to Kora's value prop.

**Tradeoff**: More upfront work than using PicoClaw as-is, but necessary for the 14-domain graph memory vision.

---

### Single binary, embedded SQLite
**Decision**: Embed koramem with SQLite + sqlite-vec in the main binary. No external database.

**Why**: Minimizes operational complexity. Backup = copy one file. Fits PicoClaw's ultra-lightweight philosophy.

**Tradeoff**: SQLite has write concurrency limits. Acceptable for single-user personal assistant.

---
