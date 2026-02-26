# sheldonmem — Memory System

> Structured domain memory with graph relationships for personal AI. Pure Go, single SQLite file.

Package: `github.com/bowerhall/sheldonmem`

## Why Build This

Existing AI memory solutions (Mem0, Zep, Letta) share the same limitations: flat unstructured facts, no domain awareness, no graph relationships, Python-only, require external vector databases. sheldonmem fixes all of this.

**What sheldonmem provides that nothing else does:**

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

Sheldon itself is a first-class entity in the store, seeded on init alongside the 14 domains:

```sql
-- Seeded on first run
INSERT INTO entities (name, entity_type, domain_id, metadata)
VALUES ('Sheldon', 'agent', 1, '{"role": "assistant", "version": "1.0"}');
```

This enables Sheldon to accumulate its own evolving identity. Facts attach to the Sheldon entity just like any other:

| field              | example value                                     | how it's learned                      |
| ------------------ | ------------------------------------------------- | ------------------------------------- |
| nickname           | "Shelly"                                          | user says "I'll call you Shelly"      |
| tone_preference    | "concise, slightly informal"                      | user feedback or explicit instruction |
| self_correction    | "I tend to over-explain career advice"            | user pushback patterns                |
| user_dynamic       | "User prefers options, not directives"            | observed interaction patterns         |
| operational_note   | "use Opus for career decisions"                   | learned from user preferences         |
| humor_style        | "dry, occasional"                                 | accumulated from positive reactions   |
| communication_lang | "English, occasional Pidgin OK"                   | user instruction                      |
| trust_level        | "autonomous for apartments, confirm for finances" | explicit user delegation              |

Edges connect Sheldon to the user and to operational concepts:

```
Sheldon →serves→ User
Sheldon →nicknamed→ "Shelly"     (stored as fact, not edge)
Sheldon →prefers_model→ Opus    (for career domain — stored as fact)
User →trusts→ Sheldon           (edge with strength, metadata: {scope: "apartment_search"})
```

**Static vs dynamic identity**: SOUL.md defines Sheldon's baseline personality at deploy time (warm, direct, culturally aware). The Sheldon entity in sheldonmem accumulates _learned_ identity — how it should adapt based on interaction history. On context assembly, both are loaded: SOUL.md first, then Sheldon entity facts layered on top as overrides.

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
    supersedes INTEGER REFERENCES facts(id),
    active BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT (datetime('now'))
);
CREATE INDEX idx_facts_domain ON facts(domain_id, active);
CREATE INDEX idx_facts_entity ON facts(entity_id, active);
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
    layer TEXT NOT NULL  -- core|inner|world|temporal|meta
);
```

### vec_facts (sqlite-vec virtual table)

Vector embeddings for semantic search over facts.

```sql
CREATE VIRTUAL TABLE vec_facts USING vec0(
    fact_id INTEGER PRIMARY KEY,
    embedding float[768]  -- dimension matches embedding model
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
- **D12 Rhythms & Routines** — daily schedule, sleep/wake, exercise, rituals, weekly patterns, **reminders**. Rate: weeks.
- **D13 Life Events & Decisions** — event log with date, category, impact, rationale. Rate: append-only.

### Layer: Meta-Awareness

- **D11 Preferences & Tastes** — food, music, aesthetics, communication style, tool preferences. Rate: years.
- **D14 Unconscious Patterns** — mannerisms, speech patterns, blind spots, external perception. Rate: years.

## Cron System

Sheldon includes a cron system (`internal/cron`) that integrates with sheldonmem for cron-augmented memory retrieval. Instead of storing reminder content in the cron, crons store keywords that search memory when they fire. This keeps memory as the source of truth.

### How It Works

```
User: "remind me to take meds every evening for 2 weeks"
                    │
                    ▼
┌─────────────────────────────────────────────┐
│ 1. Sheldon stores fact in memory (D12):     │
│    field: "medication"                       │
│    value: "take meds every evening"         │
│                                             │
│ 2. Sheldon creates cron:                    │
│    keyword: "meds"                          │
│    schedule: "0 20 * * *" (8pm daily)       │
│    expires_at: 2 weeks from now             │
└─────────────────────────────────────────────┘

[8:00 PM - CronRunner checks]
                    │
                    ▼
┌─────────────────────────────────────────────┐
│ 1. Query: WHERE next_run <= now             │
│ 2. For cron "meds":                         │
│    - Recall(ctx, "meds", nil, 10)           │
│    - Finds: "take meds every evening"       │
│ 3. Inject into agent loop with context      │
│ 4. Agent decides action: send reminder      │
│ 5. Update next_run to tomorrow 8pm          │
│ 6. After 2 weeks: auto-delete cron          │
└─────────────────────────────────────────────┘
```

### Why Keywords Instead of Storing Full Reminders

1. **Memory is the source of truth** — The cron just knows _when_ to remind, memory knows _what_ to remind about
2. **Context-aware** — When cron fires, it recalls related facts, providing richer context
3. **Natural updates** — If user says "actually, take meds at dinner not evening", memory updates automatically; cron still works
4. **No duplication** — Reminder content lives in one place (facts), schedule lives in another (crons)

### Cron API

```go
// Create a cron
cron, err := store.CreateCron("meds", "0 20 * * *", chatID, &expiresAt)

// Get due crons (next_run <= now and not expired)
dueCrons, err := store.GetDueCrons()

// Update next run after firing
err := store.UpdateCronNextRun(cronID, nextRunTime)

// List user's crons
crons, err := store.GetCronsByChat(chatID)

// Delete by keyword
err := store.DeleteCronByKeyword("meds", chatID)

// Cleanup expired crons
deleted, err := store.DeleteExpiredCrons()
```

### Cron Expressions

Standard 5-field cron format: `minute hour day-of-month month day-of-week`

| Expression    | Meaning             |
| ------------- | ------------------- |
| `0 20 * * *`  | 8pm daily           |
| `0 9 * * 1-5` | 9am weekdays        |
| `30 14 * * *` | 2:30pm daily        |
| `0 */2 * * *` | every 2 hours       |
| `0 8 1 * *`   | 8am on 1st of month |

## Notes (Two-Tier System)

Notes provide mutable state for things that change frequently and need exact key-based retrieval. Unlike facts (semantic search, may return similar results), notes guarantee exact retrieval by key.

### Two Tiers

| Tier | Purpose | Visibility | Example Keys |
|------|---------|------------|--------------|
| **Working** | Active, frequently changing state | Shown in system prompt | `current_budget`, `meal_plan`, `shopping_list` |
| **Archive** | Historical data, preserved for reference | Retrieved on-demand | `budget_2025_01`, `meal_plan_week_08` |

### When to Use Notes vs Facts

| Use Case | Storage | Why |
|----------|---------|-----|
| "I'm allergic to peanuts" | Fact | Permanent, searchable across contexts |
| "This week's meal plan" | Working Note | Mutable, exact retrieval by key |
| "January 2025 budget summary" | Archived Note | Historical record, exact retrieval |
| "I prefer TypeScript" | Fact | Stable preference, semantic recall |
| "Current month's expenses" | Working Note | Active tracking, structured JSON |

### Schema

```sql
CREATE TABLE notes (
    key TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    tier TEXT DEFAULT 'working',  -- 'working' or 'archive'
    updated_at DATETIME DEFAULT (datetime('now'))
);
CREATE INDEX idx_notes_tier ON notes(tier);
```

### How It Works

Working note keys (with age) are injected into the system prompt:

```
## Active Notes
current_budget (2 days ago), meal_plan (5 hours ago)
```

Archived notes are invisible until explicitly retrieved. Content loads only via `get_note(key)`, keeping context minimal.

### Example: Meal Planning

```
User: "Let's plan meals for the week"
                    │
                    ▼
┌─────────────────────────────────────────────┐
│ Sheldon: save_note("meal_plan", {           │
│   "week": "2026-02-24",                     │
│   "meals": {                                │
│     "monday": {"dish": "pasta", "done": false},
│     "tuesday": {"dish": "stir-fry", "done": false}
│   }                                         │
│ })                                          │
└─────────────────────────────────────────────┘

[Next day]

User: "I made the pasta"
                    │
                    ▼
┌─────────────────────────────────────────────┐
│ 1. Sheldon sees: "Active Notes: meal_plan"  │
│ 2. Calls: get_note("meal_plan")             │
│ 3. Updates monday.done = true               │
│ 4. Calls: save_note("meal_plan", updated)   │
└─────────────────────────────────────────────┘

[Later]

User: "What should I cook tonight?"
                    │
                    ▼
┌─────────────────────────────────────────────┐
│ 1. Sheldon sees: "Active Notes: meal_plan"  │
│ 2. Calls: get_note("meal_plan")             │
│ 3. Sees: monday done, tuesday pending       │
│ 4. Suggests: "You have stir-fry planned"    │
└─────────────────────────────────────────────┘
```

### Tools

| Tool | Purpose |
|------|---------|
| `save_note(key, content)` | Create or update a working note |
| `get_note(key)` | Retrieve note (searches both tiers) |
| `delete_note(key)` | Remove a note permanently |
| `archive_note(old_key, new_key)` | Move working note to archive tier |
| `list_archived_notes(pattern)` | Search archived notes by pattern |
| `restore_note(key)` | Bring archived note back to working tier |

### Lifecycle Example

```
┌─────────────────────────────────────────────────────────┐
│ January 2025                                            │
│                                                         │
│ save_note("current_budget", {...})  → Working tier      │
│ [Update throughout month]                               │
│                                                         │
│ End of month:                                           │
│ archive_note("current_budget", "budget_2025_01")        │
│ save_note("current_budget", {fresh template})           │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│ March 2025                                              │
│                                                         │
│ User: "How did I spend in January?"                     │
│ Sheldon: list_archived_notes("budget")                  │
│         → budget_2025_01, budget_2025_02                │
│ Sheldon: get_note("budget_2025_01")                     │
│         → Exact January data, no semantic matching      │
└─────────────────────────────────────────────────────────┘
```

### Integration with Crons

Notes and crons work together for time-aware state tracking:

```
User: "Give me a grocery list every Saturday morning"
                    │
                    ▼
1. Cron created: keyword="grocery", schedule="0 9 * * 6"
2. When cron fires:
   - Sheldon sees "Active Notes: meal_plan, shopping_list"
   - Recalls grocery preferences from facts
   - Checks current meal_plan note
   - Generates list based on planned meals
```

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
  SELECT * FROM facts WHERE entity_id = (Sheldon entity ID) AND active = 1
  These facts override/supplement SOUL.md in context assembly
  Loaded every request regardless of domain routing

Step 5: Return Memory{Facts, Entities, Graph, Domains, AgentSelf}
  Typically 15–30 facts + 3–8 entities + edges
  AgentSelf: always loaded — Sheldon entity facts (nickname, tone, corrections)
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
    If target == 'agent': attach to Sheldon entity (seeded on init)
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
Trigger: daily background process
Effect: low-scoring facts deprioritized in retrieval (not deleted)
```

## Memory Hygiene

| Operation     | Trigger       | Action                                       |
| ------------- | ------------- | -------------------------------------------- |
| Decay         | Daily         | Score all facts, deprioritize stale          |
| Cron cleanup  | Every minute  | Delete expired crons                         |
| Contradiction | On extraction | New fact supersedes old, old marked inactive |
| Backup        | Daily cron    | SQLite snapshot → MinIO                      |

## Cross-Domain Query Examples

**"Should I take this job offer?"**

- Router: Primary D7 (Career) + D10 (Goals). Related D8, D9, D3.
- Recall: current role, salary, professional goals, financial obligations, location preferences, stress patterns.
- Graph: employer entity → commute edge → location entity → neighborhood facts.

**"What should I eat tonight?"**

- Router: Primary D11 (Preferences) + D2 (Health).
- Recall: food preferences, dietary restrictions, allergies.
- Graph: minimal — mostly standalone facts.

**"Remind me to take my vitamins every morning"**

- Router: Primary D12 (Routines) + D2 (Health).
- Remember: stores fact in D12 with field="vitamins", value="take vitamins every morning"
- Cron: creates cron with keyword="vitamins", schedule="0 8 \* \* \*"
- When cron fires: recalls facts matching "vitamins", sends notification

## Architecture

sheldonmem is designed as a standalone, extractable package:

- Zero dependency on Sheldon or any specific assistant framework
- Pluggable interfaces for LLM (Extractor), embedding (Embedder), routing (Router)
- Single SQLite file with WAL journaling
- Pure Go with sqlite-vec for vector search
- AGPL-3.0 license

The cron system (`internal/cron`) is separate from sheldonmem, keeping memory pure. Crons use sheldonmem's database connection but maintain their own schema. This allows sheldonmem to be extracted as a standalone memory package without cron coupling.
