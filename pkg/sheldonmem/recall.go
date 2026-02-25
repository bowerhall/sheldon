package sheldonmem

import (
	"context"
	"time"
)

type RecallResult struct {
	Facts    []*Fact
	Entities []*TraversalResult
}

type RecallOptions struct {
	Depth            int        // graph traversal depth, default 1
	ExcludeSensitive bool       // if true, exclude sensitive facts from results
	Since            *time.Time // only facts created after this time
	Until            *time.Time // only facts created before this time
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

	var facts []*Fact
	var err error

	// If time range is specified and query is broad, get all facts from that time range
	isBroadQuery := query == "" || query == "*" || query == "everything" || query == "all"
	if (opts.Since != nil || opts.Until != nil) && isBroadQuery {
		since := "1970-01-01"
		until := "2100-01-01"
		if opts.Since != nil {
			since = opts.Since.Format("2006-01-02 15:04:05")
		}
		if opts.Until != nil {
			until = opts.Until.Format("2006-01-02 15:04:05")
		}
		facts, err = s.GetFactsByTimeRange(since, until, opts.ExcludeSensitive)
		if err != nil {
			return nil, err
		}
	} else {
		// Normal path: semantic/keyword search then filter
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

		// Apply time filters if specified
		if opts.Since != nil || opts.Until != nil {
			filtered := make([]*Fact, 0, len(facts))
			for _, f := range facts {
				if opts.Since != nil && f.CreatedAt.Before(*opts.Since) {
					continue
				}
				if opts.Until != nil && f.CreatedAt.After(*opts.Until) {
					continue
				}
				filtered = append(filtered, f)
			}
			facts = filtered
		}
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
