package koramem

import "context"

func (s *Store) AddFact(entityID *int64, domainID int, field, value string, confidence float64) (*Fact, error) {
	return s.AddFactWithContext(context.Background(), entityID, domainID, field, value, confidence)
}

func (s *Store) AddFactWithContext(ctx context.Context, entityID *int64, domainID int, field, value string, confidence float64) (*Fact, error) {
	// 1. Check exact field match first
	var existingID int64
	var existingValue string

	err := s.db.QueryRow(queryGetExistingFact, domainID, field, entityID).Scan(&existingID, &existingValue)

	if err == nil && existingValue != value {
		// Same field, different value → supersede
		_, err = s.db.Exec(queryDeactivateFact, existingID)
		if err != nil {
			return nil, err
		}

		s.DeleteFactEmbedding(existingID)
		return s.insertFact(ctx, entityID, domainID, field, value, confidence, &existingID)
	} else if err == nil {
		// Same field, same value → touch
		_, err = s.db.Exec(queryTouchFact, existingID)
		return &Fact{ID: existingID, EntityID: entityID, DomainID: domainID, Field: field, Value: value}, err
	}

	// 2. Check semantic similarity (different field, same meaning)
	if s.embedder != nil && entityID != nil {
		similar, err := s.findSimilarFact(ctx, *entityID, domainID, field, value)
		if err == nil && similar != nil {
			if similar.Value == value {
				// Same meaning, same value → touch existing
				_, err = s.db.Exec(queryTouchFact, similar.ID)
				return similar, err
			}
			// Same meaning, different value → supersede
			_, err = s.db.Exec(queryDeactivateFact, similar.ID)
			if err != nil {
				return nil, err
			}
			s.DeleteFactEmbedding(similar.ID)
			return s.insertFact(ctx, entityID, domainID, field, value, confidence, &similar.ID)
		}
	}

	// 3. No match → insert new
	return s.insertFact(ctx, entityID, domainID, field, value, confidence, nil)
}

func (s *Store) insertFact(ctx context.Context, entityID *int64, domainID int, field, value string, confidence float64, supersedes *int64) (*Fact, error) {
	result, err := s.db.Exec(queryInsertFact, entityID, domainID, field, value, confidence, supersedes)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()

	text := field + ": " + value
	s.EmbedFact(ctx, id, text)

	return &Fact{
		ID:         id,
		EntityID:   entityID,
		DomainID:   domainID,
		Field:      field,
		Value:      value,
		Confidence: confidence,
		Supersedes: supersedes,
		Active:     true,
	}, nil
}

func (s *Store) GetFactsByDomain(domainID int) ([]*Fact, error) {
	rows, err := s.db.Query(queryGetFactsByDomain, domainID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var facts []*Fact

	for rows.Next() {
		var f Fact
		if err := rows.Scan(&f.ID, &f.EntityID, &f.DomainID, &f.Field, &f.Value, &f.Confidence, &f.AccessCount, &f.Active, &f.CreatedAt); err != nil {
			return nil, err
		}

		facts = append(facts, &f)
	}

	return facts, nil
}

func (s *Store) GetFactsByEntity(entityID int64) ([]*Fact, error) {
	rows, err := s.db.Query(queryGetFactsByEntity, entityID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var facts []*Fact

	for rows.Next() {
		var f Fact
		if err := rows.Scan(&f.ID, &f.EntityID, &f.DomainID, &f.Field, &f.Value, &f.Confidence, &f.AccessCount, &f.Active, &f.CreatedAt); err != nil {
			return nil, err
		}

		facts = append(facts, &f)
	}

	return facts, nil
}

const similarityThreshold = 0.15 // cosine distance, lower = more similar

func (s *Store) findSimilarFact(ctx context.Context, entityID int64, domainID int, field, value string) (*Fact, error) {
	text := field + ": " + value
	embedding, err := s.embedder.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	blob, err := serializeEmbedding(embedding)
	if err != nil {
		return nil, err
	}

	// Find closest fact for same entity + domain
	row := s.db.QueryRow(`
		SELECT f.id, f.entity_id, f.domain_id, f.field, f.value, f.confidence,
		       f.access_count, f.active, f.created_at, v.distance
		FROM vec_facts v
		JOIN facts f ON v.fact_id = f.id
		WHERE f.active = 1
		  AND f.entity_id = ?
		  AND f.domain_id = ?
		  AND v.embedding MATCH ?
		  AND k = 1
		ORDER BY v.distance
		LIMIT 1
	`, entityID, domainID, blob)

	var f Fact
	var distance float32
	err = row.Scan(&f.ID, &f.EntityID, &f.DomainID, &f.Field, &f.Value, &f.Confidence,
		&f.AccessCount, &f.Active, &f.CreatedAt, &distance)
	if err != nil {
		return nil, err
	}

	if distance > similarityThreshold {
		return nil, nil // not similar enough
	}

	return &f, nil
}

func (s *Store) SearchFacts(query string, domainIDs []int) ([]*Fact, error) {
	if len(domainIDs) == 0 {
		return nil, nil
	}

	args := make([]interface{}, 0, len(domainIDs)+2)
	args = append(args, "%"+query+"%", "%"+query+"%")
	placeholders := ""
	for i, id := range domainIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, id)
	}

	rows, err := s.db.Query(querySearchFactsPrefix+placeholders+querySearchFactsSuffix, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var facts []*Fact

	for rows.Next() {
		var f Fact
		if err := rows.Scan(&f.ID, &f.EntityID, &f.DomainID, &f.Field, &f.Value, &f.Confidence, &f.AccessCount, &f.Active, &f.CreatedAt); err != nil {
			return nil, err
		}
		facts = append(facts, &f)
	}

	return facts, nil
}
