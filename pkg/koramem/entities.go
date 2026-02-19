package koramem

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
