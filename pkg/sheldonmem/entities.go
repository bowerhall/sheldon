package sheldonmem

func (s *Store) CreateEntity(name, entityType string, domainID int, metadata string) (*Entity, error) {
	result, err := s.db.Exec(queryInsertEntity, name, entityType, domainID, metadata)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()

	return &Entity{
		ID:         id,
		Name:       name,
		EntityType: entityType,
		DomainID:   domainID,
		Metadata:   metadata,
	}, nil
}

func (s *Store) GetEntity(id int64) (*Entity, error) {
	var e Entity
	row := s.db.QueryRow(queryGetEntity, id)

	err := row.Scan(&e.ID, &e.Name, &e.EntityType, &e.DomainID, &e.Metadata, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &e, nil
}

func (s *Store) FindEntityByName(name string) (*Entity, error) {
	var e Entity
	row := s.db.QueryRow(queryGetEntityByName, name)

	err := row.Scan(&e.ID, &e.Name, &e.EntityType, &e.DomainID, &e.Metadata, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &e, nil
}

func (s *Store) FindEntitiesByType(entityType string) ([]*Entity, error) {
	rows, err := s.db.Query(queryGetEntitiesByType, entityType)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var entities []*Entity

	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Name, &e.EntityType, &e.DomainID, &e.Metadata, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entities = append(entities, &e)
	}

	return entities, nil
}

func (s *Store) SearchEntities(query string) ([]*Entity, error) {
	rows, err := s.db.Query(querySearchEntities, "%"+query+"%")
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var entities []*Entity

	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Name, &e.EntityType, &e.DomainID, &e.Metadata, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entities = append(entities, &e)
	}

	return entities, nil
}

type TraversalResult struct {
	Entity   *Entity
	Relation string
	Depth    int
	Facts    []*Fact
}

func (s *Store) Traverse(entityID int64, maxDepth int) ([]*TraversalResult, error) {
	visited := make(map[int64]bool)
	var results []*TraversalResult

	var walk func(id int64, relation string, depth int) error
	walk = func(id int64, relation string, depth int) error {
		if depth > maxDepth || visited[id] {
			return nil
		}
		visited[id] = true

		entity, err := s.GetEntity(id)
		if err != nil {
			return nil
		}

		facts, _ := s.GetFactsByEntity(id)

		results = append(results, &TraversalResult{
			Entity:   entity,
			Relation: relation,
			Depth:    depth,
			Facts:    facts,
		})

		if depth < maxDepth {
			edges, err := s.getConnectedEdges(id)
			if err != nil {
				return err
			}

			for _, edge := range edges {
				targetID := edge.TargetID
				rel := edge.Relation
				if edge.TargetID == id {
					targetID = edge.SourceID
					rel = "inverse:" + rel
				}
				walk(targetID, rel, depth+1)
			}
		}

		return nil
	}

	if err := walk(entityID, "", 0); err != nil {
		return nil, err
	}

	return results, nil
}

func (s *Store) getConnectedEdges(entityID int64) ([]*Edge, error) {
	rows, err := s.db.Query(queryGetConnectedFromTo, entityID, entityID)
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
