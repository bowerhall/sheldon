# DECISIONS.md — Development Decision Log

> On-the-fly decisions and tradeoffs made during development. Newest first.

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
