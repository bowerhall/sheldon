package koramem

const (
	queryCountKoraEntity = `SELECT COUNT(*) FROM entities WHERE name = 'Kora' AND entity_type = 'agent'`

	queryInsertDomain    = `INSERT OR IGNORE INTO domains (id, name, slug, layer) VALUES (?, ?, ?, ?)`
	queryGetDomain       = `SELECT id, name, slug, layer FROM domains WHERE id = ?`
	queryGetDomainBySlug = `SELECT id, name, slug, layer FROM domains WHERE slug = ?`

	queryInsertEntity       = `INSERT INTO entities (name, entity_type, domain_id, metadata) VALUES (?, ?, ?, ?)`
	queryGetEntity          = `SELECT id, name, entity_type, domain_id, metadata, created_at, updated_at FROM entities WHERE id = ?`
	queryGetEntityByName    = `SELECT id, name, entity_type, domain_id, metadata, created_at, updated_at FROM entities WHERE name = ?`
	queryGetEntitiesByType  = `SELECT id, name, entity_type, domain_id, metadata, created_at, updated_at FROM entities WHERE entity_type = ?`
	querySearchEntities     = `SELECT id, name, entity_type, domain_id, metadata, created_at, updated_at FROM entities WHERE name LIKE ? LIMIT 10`
	queryGetConnectedFromTo = `SELECT id, source_id, target_id, relation, strength, metadata, created_at FROM edges WHERE source_id = ? OR target_id = ? ORDER BY strength DESC`

	queryGetExistingFact   = `SELECT id, value FROM facts WHERE domain_id = ? AND field = ? AND entity_id IS ? AND active = 1`
	queryDeactivateFact    = `UPDATE facts SET active = 0 WHERE id = ?`
	queryTouchFact         = `UPDATE facts SET access_count = access_count + 1, last_accessed = datetime('now') WHERE id = ?`
	queryInsertFact        = `INSERT INTO facts (entity_id, domain_id, field, value, confidence, supersedes) VALUES (?, ?, ?, ?, ?, ?)`
	queryGetFactsByDomain  = `SELECT id, entity_id, domain_id, field, value, confidence, access_count, active, created_at FROM facts WHERE domain_id = ? AND active = 1`
	queryGetFactsByEntity  = `SELECT id, entity_id, domain_id, field, value, confidence, access_count, active, created_at FROM facts WHERE entity_id = ? AND active = 1`
	querySearchFactsPrefix = `SELECT id, entity_id, domain_id, field, value, confidence, access_count, active, created_at FROM facts WHERE active = 1 AND (value LIKE ? OR field LIKE ?) AND domain_id IN (`
	querySearchFactsSuffix = `) ORDER BY (confidence * 0.7 + (1.0 / (julianday('now') - julianday(COALESCE(last_accessed, created_at)) + 1)) * 0.3) DESC LIMIT 20`

	queryInsertEdge         = `INSERT INTO edges (source_id, target_id, relation, strength, metadata) VALUES (?, ?, ?, ?, ?)`
	queryGetEdgesFrom       = `SELECT id, source_id, target_id, relation, strength, metadata, created_at FROM edges WHERE source_id = ?`
	queryGetEdgesTo         = `SELECT id, source_id, target_id, relation, strength, metadata, created_at FROM edges WHERE target_id = ?`
	queryGetEdgesByRelation = `SELECT id, source_id, target_id, relation, strength, metadata, created_at FROM edges WHERE relation = ?`

	queryInsertVecFact = `INSERT INTO vec_facts (fact_id, embedding) VALUES (?, ?)`
	queryDeleteVecFact = `DELETE FROM vec_facts WHERE fact_id = ?`
)
