# koramem — Memory System

> Structured domain memory with graph relationships for personal AI. Pure Go, single SQLite file.

Package: `github.com/kadet/koramem`

## Why Build This

Existing AI memory solutions (Mem0, Zep, Letta) share the same limitations: flat unstructured facts, no domain awareness, no graph relationships, Python-only, require external vector databases. koramem fixes all of this.

**What koramem provides that nothing else does:**
- 14 life domains as first-class schema concept
- Entity graph with typed cross-domain relationships
- Hybrid retrieval: keyword SQL + semantic vector + graph expansion
- Contradiction detection with version chains
- Decay scoring for memory hygiene
- Single SQLite file — no external services
- Pure Go — embeds in any Go binary

## Schema

### entities
First-class graph nodes: people, places, organizations, concepts, goals, events.

```sql
CREATE TABLE entities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    entity_type TEXT NOT NULL,  -- person|place|org|concept|goal|event
    domain_id INTEGER NOT NULL REFERENCES domains(id),
    metadata JSON,
    created_at DATETIME DEFAULT (datetime('now')),
    updated_at DATETIME DEFAULT (datetime('now'))
);
CREATE INDEX idx_entities_domain ON entities(domain_id);
CREATE INDEX idx_entities_type ON entities(entity_type);
CREATE INDEX idx_entities_name ON entities(name);
```

### Agent Self-Entity

Kora itself is a first-class entity in the store, seeded on init alongside the 14 domains:

```sql
-- Seeded on first run
INSERT INTO entities (name, entity_type, domain_id, metadata)
VALUES ('Kora', 'agent', 1, '{"role": "assistant", "version": "1.0"}');
```

This enables Kora to accumulate its own evolving identity. Facts attach to the Kora entity just like any other:

| field | example value | how it's learned |
|-------|--------------|-----------------|
| nickname | "K" | user says "I'll call you K" |
| tone_preference | "concise, slightly informal" | user feedback or explicit instruction |
| self_correction | "I tend to over-explain career advice" | user pushback patterns |
| user_dynamic | "Kadet prefers options, not directives" | observed interaction patterns |
| operational_note | "use Opus for career decisions" | learned from user preferences |
| humor_style | "dry, occasional" | accumulated from positive reactions |
| communication_lang | "English, occasional Pidgin OK" | user instruction |
| trust_level | "autonomous for apartments, confirm for finances" | explicit user delegation |

Edges connect Kora to the user and to operational concepts:

```
Kora →serves→ Kadet
Kora →nicknamed→ "K"            (stored as fact, not edge)
Kora →prefers_model→ Opus       (for career domain — stored as fact)
Kadet →trusts→ Kora             (edge with strength, metadata: {scope: "apartment_search"})
```

**Static vs dynamic identity**: SOUL.md defines Kora's baseline personality at deploy time (warm, direct, culturally aware). The Kora entity in koramem accumulates *learned* identity — how it should adapt based on interaction history. On context assembly, both are loaded: SOUL.md first, then Kora entity facts layered on top as overrides.

### facts
Atomic knowledge units. Domain-tagged, confidence-scored, versioned via supersedes chain.

```sql
CREATE TABLE facts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_id INTEGER REFERENCES entities(id),  -- nullable for standalone facts
    domain_id INTEGER NOT NULL REFERENCES domains(id),
    field TEXT NOT NULL,
    value TEXT NOT NULL,
    confidence REAL DEFAULT 0.8,
    access_count INTEGER DEFAULT 0,
    last_accessed DATETIME,
    source_id INTEGER REFERENCES conversations(id),
    supersedes INTEGER REFERENCES facts(id),
    active BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT (datetime('now'))
);
CREATE INDEX idx_facts_domain ON facts(domain_id, active);
CREATE INDEX idx_facts_entity ON facts(entity_id, active);
CREATE INDEX idx_facts_field ON facts(domain_id, field, active);
```

### edges
Typed relationships between entities. The graph layer that enables cross-domain reasoning.

```sql
CREATE TABLE edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id INTEGER NOT NULL REFERENCES entities(id),
    target_id INTEGER NOT NULL REFERENCES entities(id),
    relation TEXT NOT NULL,  -- works_at|lives_in|sibling_of|funds|pursues|blocked_by...
    strength REAL DEFAULT 1.0,
    metadata JSON,
    created_at DATETIME DEFAULT (datetime('now'))
);
CREATE INDEX idx_edges_source ON edges(source_id);
CREATE INDEX idx_edges_target ON edges(target_id);
CREATE INDEX idx_edges_relation ON edges(relation);
```

### domains
The 14 life domains. Seeded on init, immutable reference table.

```sql
CREATE TABLE domains (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    layer TEXT NOT NULL,  -- core|inner|world|temporal|meta
    description TEXT
);
```

### vec_facts (sqlite-vec virtual table)
Vector embeddings for semantic search over facts.

```sql
CREATE VIRTUAL TABLE vec_facts USING vec0(
    fact_id INTEGER PRIMARY KEY,
    embedding float[384]  -- dimension matches embedding model
);
```

### conversations
Source tracking for provenance.

```sql
CREATE TABLE conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id TEXT,
    summary TEXT,
    created_at DATETIME DEFAULT (datetime('now'))
);
```

## The 14 Domains

### Layer: Core Self
- **D1 Identity & Self** — name, nationality, languages, cultural identity, self-perception. Rate: years.
- **D2 Body & Health** — conditions, medications, allergies, biometrics, sleep, fitness, diet. Rate: months.

### Layer: Inner World
- **D3 Mind & Emotions** — personality traits, emotional patterns, coping, attachment style, cognitive style. Rate: months.
- **D4 Beliefs & Worldview** — religion, philosophy, politics, ethics, values, meaning. Rate: years.
- **D5 Knowledge & Skills** — education, certifications, expertise, current learning, tools. Rate: months.

### Layer: External World
- **D6 Relationships & Social** — family, partner, friends, colleagues, communities, obligations. Rate: months.
- **D7 Work & Career** — role, company, visa status, career history, professional goals, income. Rate: months.
- **D8 Finances & Assets** — income, expenses, investments, debts, budget, subscriptions. Rate: days.
- **D9 Place & Environment** — city, neighborhood, home, workspace, travel plans, possessions. Rate: months.

### Layer: Temporal
- **D10 Goals & Aspirations** — life goals, medium/short term, status, progress, blockers, deadlines. Rate: weeks.
- **D12 Rhythms & Routines** — daily schedule, sleep/wake, exercise, rituals, weekly patterns. Rate: weeks.
- **D13 Life Events & Decisions** — event log with date, category, impact, rationale. Rate: append-only.

### Layer: Meta-Awareness
- **D11 Preferences & Tastes** — food, music, aesthetics, communication style, tool preferences. Rate: years.
- **D14 Unconscious Patterns** — mannerisms, speech patterns, blind spots, external perception. Rate: years.

## Retrieval Algorithm (Recall)

```
Input: user message + Route{primary_domains, related_domains}

Step 1: Keyword search (SQL)
  For each primary domain:
    SELECT * FROM facts
    WHERE domain_id = ? AND active = 1
    AND (value LIKE '%keyword%' OR field LIKE '%keyword%')
    ORDER BY confidence DESC, access_count DESC
    LIMIT 20

  For each related domain:
    Same query, LIMIT 5

Step 2: Semantic search (sqlite-vec)
  Generate embedding for user message
  SELECT fact_id, distance FROM vec_facts
  WHERE embedding MATCH ?
  AND fact_id IN (SELECT id FROM facts WHERE domain_id IN (?) AND active = 1)
  ORDER BY distance
  LIMIT 20

Step 3: Merge + deduplicate
  Union keyword + semantic results
  Score = 0.5*confidence + 0.3*recency + 0.2*frequency
  Deduplicate by fact ID
  Sort by combined score

Step 4: Graph expansion
  For entities referenced in top facts:
    SELECT e2.*, edge.relation FROM edges
    JOIN entities e2 ON edges.target_id = e2.id
    WHERE edges.source_id = ? LIMIT 5
  Add connected entity facts to context

Step 4b: Agent self-load (always runs)
  SELECT * FROM facts WHERE entity_id = (Kora entity ID) AND active = 1
  These facts override/supplement SOUL.md in context assembly
  Loaded every request regardless of domain routing

Step 5: Return Memory{Facts, Entities, Graph, Domains, AgentSelf}
  Typically 15–30 facts + 3–8 entities + edges
  AgentSelf: always loaded — Kora entity facts (nickname, tone, corrections)
  Injected after SOUL.md as dynamic personality overrides
```

## Extraction Algorithm (Remember)

```
Input: user message + assistant response

Step 1: LLM extraction (Haiku)
  Prompt: "Extract facts, entities, and relationships from this conversation turn.
           Classify each fact as USER-DIRECTED or AGENT-DIRECTED.
           User-directed: facts about the user, their life, preferences, world.
           Agent-directed: facts about the assistant itself — nicknames, tone feedback,
             self-corrections, communication preferences, trust/autonomy levels.
           Return JSON: {facts: [{target: 'user'|'agent', domain_id, field, value, confidence, entity_name?}],
                         entities: [{name, type, domain_id}],
                         edges: [{source_name, target_name, relation}]}"

Step 2: Entity resolution
  For each extracted entity:
    Search existing entities by name (fuzzy match)
    If exists: reuse ID
    If new: INSERT into entities, generate embedding

Step 3: Fact insertion with contradiction detection
  For each extracted fact:
    If target == 'agent': attach to Kora entity (seeded on init)
    If target == 'user': attach to user entity or standalone
    Check: SELECT * FROM facts WHERE domain_id=? AND field=? AND active=1
           AND entity_id=? (matching target entity)
    If exists, same value: UPDATE access_count, last_accessed
    If exists, different value: INSERT new with supersedes=old.id, mark old active=0
    If new: INSERT, generate embedding, insert into vec_facts

Step 4: Edge creation
  For each extracted relationship:
    Resolve source/target entity IDs
    Check: existing edge with same source/target/relation?
    If exists: update strength
    If new: INSERT

Skip extraction for: confidence < 0.6, greetings, opinions, small talk.
```

## Decay Scoring

```
score = confidence × (confWeight + recencyWeight×recency + freqWeight×frequency)

Where:
  recency = 1.0 if accessed today, decays to 0.0 over staleAfter days
  frequency = min(access_count / 10, 1.0)

Default weights: confidence=0.5, recency=0.3, frequency=0.2
Default staleAfter: 90 days

Run via: store.Decay(ctx, config)
Trigger: weekly cron job
Effect: low-scoring facts deprioritized in retrieval (not deleted)
```

## Memory Hygiene

| Operation | Trigger | Action |
|-----------|---------|--------|
| Decay | Weekly cron | Score all facts, deprioritize stale |
| Compact | Weekly cron | Merge redundant facts, clean superseded chains |
| Contradiction | On extraction | New fact supersedes old, old marked inactive |
| Backup | Daily cron | SQLite snapshot → MinIO |
| Audit | On demand | `/review` command — list recent facts for approval |
| Gaps | Monthly cron | Check domain coverage, prompt for sparse domains |

## Cross-Domain Query Examples

**"Should I take this job offer?"**
- Router: Primary D7 (Career) + D10 (Goals). Related D8, D9, D3.
- Recall: current role, salary, professional goals, financial obligations, location preferences, stress patterns.
- Graph: employer entity → commute edge → location entity → neighborhood facts.

**"What should I eat tonight?"**
- Router: Primary D11 (Preferences) + D2 (Health).
- Recall: food preferences, dietary restrictions, allergies.
- Graph: minimal — mostly standalone facts.

**"Help me plan my OMSCS application"**
- Router: Primary D10 (Goals) + D5 (Knowledge). Related D7, D8, D13.
- Recall: OMSCS deadline, education history, current skills, career trajectory, application costs.
- Graph: Georgia Tech entity → offers edge → OMSCS entity → deadline/requirements facts.

## Open-Source Strategy

koramem is designed as a standalone, extractable package:
- Zero dependency on Kora, PicoClaw, or any specific assistant framework
- Pluggable interfaces for LLM (Extractor), embedding (Embedder), routing (Router)
- Ship with default implementations for Claude + Voyage AI
- MIT license
- README with examples for: personal assistant, chatbot memory, note-taking app
