package koramem

import (
	"context"
	"database/sql"
)

func (s *Store) AddFact(entityID *int64, domainID int, field, value string, confidence float64) (*Fact, error) {
	return s.AddFactWithContext(context.Background(), entityID, domainID, field, value, confidence)
}

func (s *Store) AddFactWithContext(ctx context.Context, entityID *int64, domainID int, field, value string, confidence float64) (*Fact, error) {
	var existingID int64
	var existingValue string

	err := s.db.QueryRow(queryGetExistingFact, domainID, field, entityID).Scan(&existingID, &existingValue)

	if err == nil && existingValue != value {
		_, err = s.db.Exec(queryDeactivateFact, existingID)
		if err != nil {
			return nil, err
		}

		s.DeleteFactEmbedding(existingID)
		return s.insertFact(ctx, entityID, domainID, field, value, confidence, &existingID)
	} else if err == sql.ErrNoRows {
		return s.insertFact(ctx, entityID, domainID, field, value, confidence, nil)
	} else if err == nil {
		_, err = s.db.Exec(queryTouchFact, existingID)

		return &Fact{ID: existingID, EntityID: entityID, DomainID: domainID, Field: field, Value: value}, err
	}

	return nil, err
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
