package koramem

import (
	"context"
	"fmt"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/ncruces"
)

func serializeEmbedding(embedding []float32) ([]byte, error) {
	return sqlite_vec.SerializeFloat32(embedding)
}

func (s *Store) EmbedFact(ctx context.Context, factID int64, text string) error {
	if s.embedder == nil {
		return nil
	}

	embedding, err := s.embedder.Embed(ctx, text)
	if err != nil {
		return err
	}

	blob, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(queryInsertVecFact, factID, blob)
	return err
}

func (s *Store) DeleteFactEmbedding(factID int64) error {
	_, err := s.db.Exec(queryDeleteVecFact, factID)
	return err
}

type ScoredFact struct {
	Fact     *Fact
	Distance float32
}

func (s *Store) SemanticSearch(ctx context.Context, query string, domainIDs []int, limit int) ([]*ScoredFact, error) {
	if s.embedder == nil {
		return nil, nil
	}

	if len(domainIDs) == 0 {
		return nil, nil
	}

	embedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	blob, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return nil, err
	}

	placeholders := ""
	args := make([]interface{}, 0, len(domainIDs)+2)
	args = append(args, blob, limit)

	for i, id := range domainIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, id)
	}

	q := fmt.Sprintf(`
		SELECT f.id, f.entity_id, f.domain_id, f.field, f.value, f.confidence,
		       f.access_count, f.active, f.created_at, v.distance
		FROM vec_facts v
		JOIN facts f ON v.fact_id = f.id
		WHERE f.active = 1
		  AND v.embedding MATCH ?
		  AND k = ?
		  AND f.domain_id IN (%s)
		ORDER BY v.distance
	`, placeholders)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*ScoredFact
	for rows.Next() {
		var f Fact
		var distance float32
		if err := rows.Scan(&f.ID, &f.EntityID, &f.DomainID, &f.Field, &f.Value, &f.Confidence, &f.AccessCount, &f.Active, &f.CreatedAt, &distance); err != nil {
			return nil, err
		}
		results = append(results, &ScoredFact{Fact: &f, Distance: distance})
	}

	return results, nil
}

func (s *Store) HybridSearch(ctx context.Context, query string, domainIDs []int, limit int) ([]*Fact, error) {
	keywordFacts, err := s.SearchFacts(query, domainIDs)
	if err != nil {
		return nil, err
	}

	if s.embedder == nil {
		if len(keywordFacts) > limit {
			keywordFacts = keywordFacts[:limit]
		}
		return keywordFacts, nil
	}

	semanticFacts, err := s.SemanticSearch(ctx, query, domainIDs, limit)
	if err != nil {
		if len(keywordFacts) > limit {
			keywordFacts = keywordFacts[:limit]
		}
		return keywordFacts, nil
	}

	seen := make(map[int64]bool)
	var results []*Fact

	for _, sf := range semanticFacts {
		if !seen[sf.Fact.ID] {
			seen[sf.Fact.ID] = true
			results = append(results, sf.Fact)
		}
	}

	for _, f := range keywordFacts {
		if !seen[f.ID] {
			seen[f.ID] = true
			results = append(results, f)
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (s *Store) ReindexEmbeddings(ctx context.Context) error {
	if s.embedder == nil {
		return nil
	}

	rows, err := s.db.Query(`SELECT id, field, value FROM facts WHERE active = 1`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var field, value string
		if err := rows.Scan(&id, &field, &value); err != nil {
			return err
		}

		text := field + ": " + value
		if err := s.EmbedFact(ctx, id, text); err != nil {
			return err
		}
	}

	return nil
}
