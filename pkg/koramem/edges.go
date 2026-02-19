package koramem

func (s *Store) AddEdge(sourceID, targetID int64, relation string, strength float64, metadata string) (*Edge, error) {
	result, err := s.db.Exec(queryInsertEdge, sourceID, targetID, relation, strength, metadata)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &Edge{
		ID:       id,
		SourceID: sourceID,
		TargetID: targetID,
		Relation: relation,
		Strength: strength,
		Metadata: metadata,
	}, nil
}

func (s *Store) GetEdgesFrom(entityID int64) ([]*Edge, error) {
	rows, err := s.db.Query(queryGetEdgesFrom, entityID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var edges []*Edge

	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.ID, &e.SourceID, &e.TargetID, &e.Relation, &e.Strength, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}

		edges = append(edges, &e)
	}

	return edges, nil
}

func (s *Store) GetEdgesTo(entityID int64) ([]*Edge, error) {
	rows, err := s.db.Query(queryGetEdgesTo, entityID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var edges []*Edge

	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.ID, &e.SourceID, &e.TargetID, &e.Relation, &e.Strength, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}

		edges = append(edges, &e)
	}

	return edges, nil
}

func (s *Store) GetEdgesByRelation(relation string) ([]*Edge, error) {
	rows, err := s.db.Query(queryGetEdgesByRelation, relation)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var edges []*Edge

	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.ID, &e.SourceID, &e.TargetID, &e.Relation, &e.Strength, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}

		edges = append(edges, &e)
	}

	return edges, nil
}
