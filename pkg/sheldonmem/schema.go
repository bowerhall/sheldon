package sheldonmem

const VectorDimensions = 768

const schema = `
CREATE TABLE IF NOT EXISTS domains (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    layer TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS entities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    domain_id INTEGER NOT NULL REFERENCES domains(id),
    metadata TEXT,
    created_at DATETIME DEFAULT (datetime('now')),
    updated_at DATETIME DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_entities_domain ON entities(domain_id);
CREATE INDEX IF NOT EXISTS idx_entities_type ON entities(entity_type);
CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name);

CREATE TABLE IF NOT EXISTS facts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_id INTEGER REFERENCES entities(id),
    domain_id INTEGER NOT NULL REFERENCES domains(id),
    field TEXT NOT NULL,
    value TEXT NOT NULL,
    confidence REAL DEFAULT 0.8,
    access_count INTEGER DEFAULT 0,
    last_accessed DATETIME,
    supersedes INTEGER REFERENCES facts(id),
    active INTEGER DEFAULT 1,
    sensitive INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_facts_domain ON facts(domain_id, active);
CREATE INDEX IF NOT EXISTS idx_facts_entity ON facts(entity_id, active);

CREATE TABLE IF NOT EXISTS edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id INTEGER NOT NULL REFERENCES entities(id),
    target_id INTEGER NOT NULL REFERENCES entities(id),
    relation TEXT NOT NULL,
    strength REAL DEFAULT 1.0,
    metadata TEXT,
    created_at DATETIME DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_edges_source ON edges(source_id);
CREATE INDEX IF NOT EXISTS idx_edges_target ON edges(target_id);
CREATE INDEX IF NOT EXISTS idx_edges_relation ON edges(relation);

CREATE TABLE IF NOT EXISTS notes (
    key TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    tier TEXT DEFAULT 'working',
    updated_at DATETIME DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS conversation_chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_chunks_session ON conversation_chunks(session_id);
CREATE INDEX IF NOT EXISTS idx_chunks_date ON conversation_chunks(date(created_at));

CREATE TABLE IF NOT EXISTS daily_summaries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    summary_date DATE NOT NULL,
    summary TEXT NOT NULL,
    created_at DATETIME DEFAULT (datetime('now')),
    UNIQUE(session_id, summary_date)
);

CREATE INDEX IF NOT EXISTS idx_summaries_session ON daily_summaries(session_id);
CREATE INDEX IF NOT EXISTS idx_summaries_date ON daily_summaries(summary_date);
`

const vecSchema = `
CREATE VIRTUAL TABLE IF NOT EXISTS vec_facts USING vec0(
    fact_id INTEGER PRIMARY KEY,
    embedding FLOAT[768]
);

CREATE VIRTUAL TABLE IF NOT EXISTS vec_summaries USING vec0(
    summary_id INTEGER PRIMARY KEY,
    embedding FLOAT[768]
);
`
