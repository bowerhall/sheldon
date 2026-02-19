# Glossary

| Term | Definition |
|------|-----------|
| **koramem** | Go package for structured domain memory with graph relationships. SQLite + sqlite-vec. |
| **Domain** | One of 14 life categories (Identity, Health, Career, etc.) used to organize knowledge. |
| **Entity** | A first-class node in the knowledge graph: a person, place, org, concept, goal, or event. |
| **Fact** | An atomic knowledge unit: a domain-tagged, confidence-scored attribute or observation. |
| **Edge** | A typed relationship between two entities (works_at, lives_in, funds, pursues, etc.). |
| **Domain Router** | Haiku LLM call that classifies a message into primary + related domains and selects model tier. |
| **Recall** | koramem's retrieval operation: hybrid keyword + semantic + graph expansion search. |
| **Remember** | koramem's storage operation: LLM extraction → entity resolution → fact insertion. |
| **Extractor** | Pluggable interface for extracting facts/entities/edges from conversation turns. |
| **Embedder** | Pluggable interface for generating vector embeddings for semantic search. |
| **PicoClaw** | Ultra-lightweight Go AI assistant framework. Kora is a fork of PicoClaw. |
| **SOUL.md** | File defining Kora's personality, tone, and behavioral guidelines. |
| **IDENTITY.md** | File with seeded facts about the user (bootstrap for empty memory). |
| **Supersedes** | When a new fact contradicts an existing one, the new fact supersedes the old. |
| **Agent Entity** | Kora's own entity in koramem — stores evolving identity (nickname, tone preferences, self-corrections, learned user dynamics). Seeded on init. |
| **Agent-Directed Fact** | A fact about the assistant itself, extracted from conversation and attached to the Kora entity. Contrast with user-directed facts. |
| **User-Directed Fact** | A fact about the user or their world, attached to user entities or standalone. The default extraction target. |
| **Self-Load** | The step in every Recall where Kora entity facts are always fetched regardless of domain routing, providing dynamic personality overrides. |
| **Decay** | Process of deprioritizing facts that haven't been accessed recently. |
| **sqlite-vec** | SQLite extension for vector similarity search (KNN, cosine distance). |
| **Skill** | A SKILL.md file that gives Kora instructions for a specific capability. |
| **Claude Code Bridge** | The subsystem connecting Kora's conversational agent loop to Claude Code's operational agent loop. Handles invocation via Go Agent SDK, context bridging (CLAUDE.md generation from koramem), sandboxing, complexity tiers, multi-pass orchestration, and output sanitization. See [claude-code-bridge.md](claude-code-bridge.md). |
| **Strategy Engine** | Skill that applies multi-framework decision analysis (BATNA, regret min, etc.). |
| **k3s** | Lightweight Kubernetes distribution for single-node clusters. |
| **WAL** | Write-Ahead Logging — SQLite mode enabling concurrent reads with single writer. |
