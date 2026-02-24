package sheldonmem

import "context"

type RecallResult struct {
	Facts    []*Fact
	Entities []*TraversalResult
}

type RecallOptions struct {
	Depth            int  // graph traversal depth, default 1
	ExcludeSensitive bool // if true, exclude sensitive facts from results
}

func (s *Store) Recall(ctx context.Context, query string, domainIDs []int, limit int) (*RecallResult, error) {
	return s.RecallWithOptions(ctx, query, domainIDs, limit, RecallOptions{Depth: 1})
}

func (s *Store) RecallWithOptions(ctx context.Context, query string, domainIDs []int, limit int, opts RecallOptions) (*RecallResult, error) {
	result := &RecallResult{}

	depth := opts.Depth
	if depth < 1 {
		depth = 1
	}
	if depth > 3 {
		depth = 3 // cap to prevent excessive traversal
	}

	// 1. Hybrid search for facts (semantic + keyword)
	var facts []*Fact
	var err error
	if opts.ExcludeSensitive {
		facts, err = s.HybridSearchSafe(ctx, query, domainIDs, limit)
		if err != nil {
			facts, err = s.SearchFactsSafe(query, domainIDs)
		}
	} else {
		facts, err = s.HybridSearch(ctx, query, domainIDs, limit)
		if err != nil {
			facts, err = s.SearchFacts(query, domainIDs)
		}
	}
	if err != nil {
		return nil, err
	}
	result.Facts = facts

	// 2. Search for entities matching the query
	entities, err := s.SearchEntities(query)
	if err != nil {
		return result, nil
	}

	// 3. Traverse from each found entity with specified depth
	seen := make(map[int64]bool)
	for _, entity := range entities {
		if seen[entity.ID] {
			continue
		}

		traversal, err := s.Traverse(entity.ID, depth)
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
