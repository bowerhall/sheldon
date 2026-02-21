# Glossary

| Term | Definition |
|------|-----------|
| **sheldonmem** | Go package for structured domain memory with graph relationships. SQLite + sqlite-vec. |
| **Domain** | One of 14 life categories (Identity, Health, Career, etc.) used to organize knowledge. |
| **Entity** | A first-class node in the knowledge graph: a person, place, org, concept, goal, or event. |
| **Fact** | An atomic knowledge unit: a domain-tagged, confidence-scored attribute or observation. |
| **Edge** | A typed relationship between two entities (works_at, lives_in, funds, pursues, etc.). |
| **Domain Router** | Haiku LLM call that classifies a message into primary + related domains and selects model tier. |
| **Recall** | sheldonmem's retrieval operation: hybrid keyword + semantic + graph expansion search. |
| **Remember** | sheldonmem's storage operation: LLM extraction → entity resolution → fact insertion. |
| **Extractor** | Pluggable interface for extracting facts/entities/edges from conversation turns. |
| **Embedder** | Pluggable interface for generating vector embeddings for semantic search. |
| **SOUL.md** | File defining Sheldon's personality, tone, and behavioral guidelines. |
| **IDENTITY.md** | File with seeded facts about the user (bootstrap for empty memory). |
| **Supersedes** | When a new fact contradicts an existing one, the new fact supersedes the old. |
| **Agent Entity** | Sheldon's own entity in sheldonmem — stores evolving identity (nickname, tone preferences, self-corrections, learned user dynamics). Seeded on init. |
| **Agent-Directed Fact** | A fact about the assistant itself, extracted from conversation and attached to the Sheldon entity. Contrast with user-directed facts. |
| **User-Directed Fact** | A fact about the user or their world, attached to user entities or standalone. The default extraction target. |
| **Self-Load** | The step in every Recall where Sheldon entity facts are always fetched regardless of domain routing, providing dynamic personality overrides. |
| **Decay** | Process of deprioritizing facts that haven't been accessed recently. |
| **sqlite-vec** | SQLite extension for vector similarity search (KNN, cosine distance). |
| **Skill** | A markdown file that gives Sheldon instructions for a specific capability. |
| **Coder Bridge** | The subsystem connecting Sheldon's conversational agent loop to the Coder's operational agent loop. Handles invocation, context bridging (CONTEXT.md generation from sheldonmem), sandboxing, complexity tiers, multi-pass orchestration, and output sanitization. See [coder-bridge.md](coder-bridge.md). |
| **Strategy Engine** | Skill that applies multi-framework decision analysis (BATNA, regret min, etc.). |
| **Docker Compose** | Container orchestration tool used for deploying Sheldon and apps. |
| **Traefik** | Reverse proxy for routing traffic to containers via labels. |
| **WAL** | Write-Ahead Logging — SQLite mode enabling concurrent reads with single writer. |
