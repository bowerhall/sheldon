# Phase 0: The Brain + sheldonmem

**Timeline: 1-2 weeks**
**Depends on: Nothing**
**Goal: Telegram bot with 14-domain graph memory in a single Go binary**

## Why Build sheldonmem

Standard AI assistants use flat text files for memory. This works for basic context but lacks structured domains, semantic search, entity relationships, and contradiction detection. Building sheldonmem gives us: 14 life domains as first-class schema, hybrid retrieval with sqlite-vec, entity graph with typed edges, and an open-sourceable package we fully control.

## Tasks

### 1. Bootstrap Project (Day 1)
- Create fresh Go project structure
- Verify builds: `go build ./cmd/sheldon`
- Configure Telegram bot token, Claude API key
- Test basic message → response flow

### 2. sheldonmem Schema + Store (Days 2-3)
- Create `pkg/sheldonmem/` package directory
- Implement schema.go: DDL for entities, facts, edges, domains, conversations, vec_facts
- Implement store.go: `Open()`, `Close()`, migrations, WAL mode enable
- Implement domains.go: seed 14 domains on first init
- Seed Sheldon agent entity: `{name:"Sheldon", type:"agent", domain_id:1}` — created on init alongside domains
- SQLite driver: `modernc.org/sqlite` (pure Go) or `mattn/go-sqlite3` + sqlite-vec CGo
- Tests: schema creation, domain seeding, Sheldon entity seeding, basic CRUD

### 3. sheldonmem Entity + Fact CRUD (Days 3-4)
- entities.go: `CreateEntity()`, `GetEntity()`, `FindEntities()`
- facts.go: `AddFact()`, `GetFacts()`, `SearchFacts()` (keyword SQL only first)
- Contradiction detection: `AddFact` checks existing field, handles supersedes chain
- edges.go: `AddEdge()`, `GetEdges()`
- Tests: entity creation, fact CRUD, contradiction handling, edge operations

### 4. sqlite-vec Integration (Days 4-5)
- Install sqlite-vec extension
- Implement embedding storage in vec_facts virtual table
- Implement semantic search: KNN query on vec_facts with domain filtering
- Implement hybrid merge: combine keyword + semantic results, score, deduplicate
- Embedder interface + stub implementation (for testing without API calls)
- Tests: vector storage, KNN retrieval, hybrid search accuracy

### 5. Graph Traversal (Day 5-6)
- Implement `Traverse()`: BFS from entity, configurable depth + relation filters
- Implement graph expansion in Recall: for top entities, pull 1-hop connected entities
- Return `Graph` struct with Nodes + Edges for context assembly
- Tests: multi-hop traversal, cross-domain edge following

### 6. Remember + Recall (Days 6-8)
- Implement `Recall()`: Route → keyword search → semantic search → graph expand → merge
- Implement agent self-load: always query Sheldon entity facts, inject after SOUL.md
- Implement `Remember()`: LLM extraction → entity resolution → fact insertion → embedding
- Extraction prompt distinguishes user-directed vs agent-directed facts
- Agent-directed facts (nickname, tone feedback, corrections) attach to Sheldon entity
- Extractor interface + Claude Haiku implementation
- Embedder interface + Voyage AI implementation
- Router interface + Claude Haiku implementation
- Tests: end-to-end Remember → Recall cycle, agent-directed fact storage, self-load

### 7. Integrate into Sheldon (Days 8-10)
- Create ContextBuilder: call `store.Recall()` and inject facts into context
- Add post-response hook: call `store.Remember()` async after each response
- Add Domain Router as pre-processing step in agent loop
- Create SOUL.md and IDENTITY.md files, loaded into every context
- Add sheldonmem.Recall results to ContextBuilder output
- Test full flow: message → route → recall → LLM → respond → remember

### 8. Deploy to VPS (Days 10-12)
- Set up Hetzner CX32 VPS
- Install Docker
- Build Docker image (single Go binary + sqlite-vec + sheldon.db volume)
- Create docker-compose.yml with Traefik
- Deploy and test end-to-end via Telegram

## Success Criteria

- [ ] Single Go binary runs as Telegram bot
- [ ] sheldonmem stores entities, facts, edges in SQLite
- [ ] Semantic search works via sqlite-vec
- [ ] Graph traversal returns cross-domain connected entities
- [ ] Remember extracts facts from conversation turns
- [ ] Recall retrieves relevant facts for new messages
- [ ] Contradiction detection supersedes old facts
- [ ] Running on Docker Compose with persistent storage
- [ ] Can have a multi-turn conversation where Sheldon remembers facts from earlier turns
- [ ] Sheldon entity seeded on init, agent-directed facts stored correctly
- [ ] Giving Sheldon a nickname persists and is reflected in subsequent responses
