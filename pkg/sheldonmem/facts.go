package sheldonmem

import (
	"context"
	"fmt"
	"strings"
)

func (s *Store) AddFact(entityID *int64, domainID int, field, value string, confidence float64) (*FactResult, error) {
	return s.AddFactWithContext(context.Background(), entityID, domainID, field, value, confidence, false)
}

func (s *Store) AddSensitiveFact(entityID *int64, domainID int, field, value string, confidence float64) (*FactResult, error) {
	return s.AddFactWithContext(context.Background(), entityID, domainID, field, value, confidence, true)
}

func (s *Store) AddFactWithContext(ctx context.Context, entityID *int64, domainID int, field, value string, confidence float64, sensitive bool) (*FactResult, error) {
	// 1. Check exact field match first
	var existingID int64
	var existingValue string

	err := s.db.QueryRow(queryGetExistingFact, domainID, field, entityID).Scan(&existingID, &existingValue)

	if err == nil && existingValue != value {
		// Same field, different value → supersede
		superseded := &Fact{ID: existingID, EntityID: entityID, DomainID: domainID, Field: field, Value: existingValue}

		_, err = s.db.Exec(queryDeactivateFact, existingID)
		if err != nil {
			return nil, err
		}

		s.DeleteFactEmbedding(existingID)
		fact, err := s.insertFact(ctx, entityID, domainID, field, value, confidence, &existingID, sensitive)
		if err != nil {
			return nil, err
		}
		return &FactResult{Fact: fact, Superseded: superseded}, nil
	} else if err == nil {
		// Same field, same value → touch
		_, err = s.db.Exec(queryTouchFact, existingID)
		return &FactResult{Fact: &Fact{ID: existingID, EntityID: entityID, DomainID: domainID, Field: field, Value: value}}, err
	}

	// 2. Check semantic similarity (different field, same meaning)
	if s.embedder != nil && entityID != nil {
		similar, err := s.findSimilarFact(ctx, *entityID, domainID, field, value)
		if err == nil && similar != nil {
			if similar.Value == value {
				// Same meaning, same value → touch existing
				_, err = s.db.Exec(queryTouchFact, similar.ID)
				return &FactResult{Fact: similar}, err
			}
			// Same meaning, different value → supersede
			superseded := &Fact{ID: similar.ID, EntityID: similar.EntityID, DomainID: similar.DomainID, Field: similar.Field, Value: similar.Value}

			_, err = s.db.Exec(queryDeactivateFact, similar.ID)
			if err != nil {
				return nil, err
			}
			s.DeleteFactEmbedding(similar.ID)
			fact, err := s.insertFact(ctx, entityID, domainID, field, value, confidence, &similar.ID, sensitive)
			if err != nil {
				return nil, err
			}
			return &FactResult{Fact: fact, Superseded: superseded}, nil
		}
	}

	// 3. No match → insert new
	fact, err := s.insertFact(ctx, entityID, domainID, field, value, confidence, nil, sensitive)
	if err != nil {
		return nil, err
	}
	return &FactResult{Fact: fact}, nil
}

func (s *Store) insertFact(ctx context.Context, entityID *int64, domainID int, field, value string, confidence float64, supersedes *int64, sensitive bool) (*Fact, error) {
	result, err := s.db.Exec(queryInsertFact, entityID, domainID, field, value, confidence, supersedes, sensitive)
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
		Sensitive:  sensitive,
	}, nil
}

// MarkSensitive marks a fact as sensitive or not
func (s *Store) MarkSensitive(factID int64, sensitive bool) error {
	_, err := s.db.Exec(queryMarkSensitive, sensitive, factID)
	return err
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
		if err := rows.Scan(&f.ID, &f.EntityID, &f.DomainID, &f.Field, &f.Value, &f.Confidence, &f.AccessCount, &f.Active, &f.Sensitive, &f.CreatedAt); err != nil {
			return nil, err
		}

		facts = append(facts, &f)
	}

	if err := rows.Err(); err != nil {
		return nil, err
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
		if err := rows.Scan(&f.ID, &f.EntityID, &f.DomainID, &f.Field, &f.Value, &f.Confidence, &f.AccessCount, &f.Active, &f.Sensitive, &f.CreatedAt); err != nil {
			return nil, err
		}

		facts = append(facts, &f)
	}

	if err := rows.Err(); err != nil {
		return nil, err
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

// GetSupersededFacts returns previous values for a field (inactive facts)
func (s *Store) GetSupersededFacts(field string, entityID *int64) ([]*Fact, error) {
	rows, err := s.db.Query(queryGetSupersededFacts, field, entityID)
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

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return facts, nil
}

func (s *Store) SearchFacts(query string, domainIDs []int) ([]*Fact, error) {
	return s.searchFacts(query, domainIDs, false)
}

func (s *Store) GetFactsByTimeRange(since, until string, excludeSensitive bool) ([]*Fact, error) {
	query := queryGetFactsByTimeRange
	if excludeSensitive {
		query = queryGetFactsByTimeRangeSafe
	}

	rows, err := s.db.Query(query, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var facts []*Fact
	for rows.Next() {
		var f Fact
		if err := rows.Scan(&f.ID, &f.EntityID, &f.DomainID, &f.Field, &f.Value, &f.Confidence, &f.AccessCount, &f.Active, &f.Sensitive, &f.CreatedAt); err != nil {
			return nil, err
		}
		facts = append(facts, &f)
	}
	return facts, rows.Err()
}

// SearchFactsSafe searches facts but excludes sensitive ones
func (s *Store) SearchFactsSafe(query string, domainIDs []int) ([]*Fact, error) {
	return s.searchFacts(query, domainIDs, true)
}

func (s *Store) searchFacts(query string, domainIDs []int, excludeSensitive bool) ([]*Fact, error) {
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

	prefix := querySearchFactsPrefix
	if excludeSensitive {
		prefix = querySearchFactsSafePrefix
	}

	rows, err := s.db.Query(prefix+placeholders+querySearchFactsSuffix, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var facts []*Fact

	for rows.Next() {
		var f Fact
		if err := rows.Scan(&f.ID, &f.EntityID, &f.DomainID, &f.Field, &f.Value, &f.Confidence, &f.AccessCount, &f.Active, &f.Sensitive, &f.CreatedAt); err != nil {
			return nil, err
		}
		facts = append(facts, &f)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return facts, nil
}

// TouchFacts increments access_count and updates last_accessed for the given fact IDs.
// This tracks salience - frequently accessed facts are more important.
func (s *Store) TouchFacts(factIDs []int64) error {
	if len(factIDs) == 0 {
		return nil
	}

	placeholders := make([]string, len(factIDs))
	args := make([]interface{}, len(factIDs))
	for i, id := range factIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(queryTouchFacts, strings.Join(placeholders, ","))
	_, err := s.db.Exec(query, args...)
	return err
}
