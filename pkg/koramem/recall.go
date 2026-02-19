package koramem

import "context"

type RecallResult struct {
	Facts    []*Fact
	Entities []*TraversalResult
}

func (s *Store) Recall(ctx context.Context, query string, domainIDs []int, limit int) (*RecallResult, error) {
	result := &RecallResult{}

	// 1. Hybrid search for facts (semantic + keyword)
	facts, err := s.HybridSearch(ctx, query, domainIDs, limit)
	if err != nil {
		// Fall back to keyword search
		facts, err = s.SearchFacts(query, domainIDs)
		if err != nil {
			return nil, err
		}
	}
	result.Facts = facts

	// 2. Search for entities matching the query
	entities, err := s.SearchEntities(query)
	if err != nil {
		return result, nil
	}

	// 3. Traverse from each found entity (depth 1 = direct connections only)
	seen := make(map[int64]bool)
	for _, entity := range entities {
		if seen[entity.ID] {
			continue
		}

		traversal, err := s.Traverse(entity.ID, 1)
		if err != nil {
			continue
		}

		for _, t := range traversal {
			if !seen[t.Entity.ID] {
				seen[t.Entity.ID] = true
				result.Entities = append(result.Entities, t)
			}
		}
	}

	return result, nil
}
