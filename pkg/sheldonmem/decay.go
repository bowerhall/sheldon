package sheldonmem

import (
	"fmt"
	"strings"
)

// Decay removes low-salience facts using a combined score of:
// - Age (days since created)
// - Recency (days since last accessed)
// - Access count (how often the fact has been recalled)
//
// Salience formula: (recency_score * 0.4) + (access_score * 0.4) + (confidence * 0.2)
// Facts with salience below threshold AND older than MaxAge are deleted.
func (s *Store) Decay(cfg DecayConfig) (int64, error) {
	if cfg.MaxAge == 0 {
		cfg = DefaultDecayConfig
	}
	if cfg.SalienceThreshold == 0 {
		cfg.SalienceThreshold = 0.2
	}

	defaultDays := int(cfg.MaxAge.Hours() / 24)

	// Salience score calculation:
	// - recency_score: 1.0 if accessed today, decays to 0 over 90 days
	// - access_score: min(access_count / 10, 1.0) - caps at 10 accesses
	// - confidence: direct value (0-1)
	//
	// salience = (recency * 0.4) + (access * 0.4) + (confidence * 0.2)
	salienceSQL := `
		(
			-- recency score: 1.0 if accessed recently, decays over 90 days
			MAX(0, 1.0 - (julianday('now') - julianday(COALESCE(last_accessed, created_at))) / 90.0) * 0.4
			+
			-- access score: caps at 10 accesses
			MIN(access_count / 10.0, 1.0) * 0.4
			+
			-- confidence score
			confidence * 0.2
		)
	`

	var conditions []string
	var args []interface{}

	// Age condition with domain overrides
	if len(cfg.DomainOverrides) > 0 {
		var domainCases []string
		for domainID, maxAge := range cfg.DomainOverrides {
			days := int(maxAge.Hours() / 24)
			domainCases = append(domainCases,
				fmt.Sprintf("(domain_id = %d AND created_at < datetime('now', '-%d days'))", domainID, days))
		}

		overrideDomains := make([]string, 0, len(cfg.DomainOverrides))
		for domainID := range cfg.DomainOverrides {
			overrideDomains = append(overrideDomains, fmt.Sprintf("%d", domainID))
		}

		domainCases = append(domainCases,
			fmt.Sprintf("(domain_id NOT IN (%s) AND created_at < datetime('now', '-%d days'))",
				strings.Join(overrideDomains, ","), defaultDays))

		conditions = append(conditions, "("+strings.Join(domainCases, " OR ")+")")
	} else {
		conditions = append(conditions, fmt.Sprintf("created_at < datetime('now', '-%d days')", defaultDays))
	}

	// Salience threshold condition
	conditions = append(conditions, fmt.Sprintf("%s < ?", salienceSQL))
	args = append(args, cfg.SalienceThreshold)

	// Delete embeddings first
	deleteVecQuery := fmt.Sprintf(`
		DELETE FROM vec_facts WHERE fact_id IN (
			SELECT id FROM facts WHERE active = 1 AND %s
		)
	`, strings.Join(conditions, " AND "))

	s.db.Exec(deleteVecQuery, args...)

	// Delete facts
	deleteQuery := fmt.Sprintf(`
		DELETE FROM facts WHERE active = 1 AND %s
	`, strings.Join(conditions, " AND "))

	result, err := s.db.Exec(deleteQuery, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// GetFactSalience returns the salience score for a fact (0-1, higher = more important)
func (s *Store) GetFactSalience(factID int64) (float64, error) {
	var salience float64
	err := s.db.QueryRow(`
		SELECT
			MAX(0, 1.0 - (julianday('now') - julianday(COALESCE(last_accessed, created_at))) / 90.0) * 0.4
			+ MIN(access_count / 10.0, 1.0) * 0.4
			+ confidence * 0.2
		FROM facts WHERE id = ?
	`, factID).Scan(&salience)
	return salience, err
}
