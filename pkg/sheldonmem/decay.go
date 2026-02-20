package sheldonmem

import (
	"fmt"
	"strings"
)

func (s *Store) Decay(cfg DecayConfig) (int64, error) {
	if cfg.MaxAge == 0 {
		cfg = DefaultDecayConfig
	}

	defaultDays := int(cfg.MaxAge.Hours() / 24)

	var conditions []string
	var args []interface{}

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

	conditions = append(conditions, "access_count <= ?")
	args = append(args, cfg.MaxAccessCount)

	conditions = append(conditions, "confidence <= ?")
	args = append(args, cfg.MaxConfidence)

	// delete embeddings first
	deleteVecQuery := fmt.Sprintf(`
		DELETE FROM vec_facts WHERE fact_id IN (
			SELECT id FROM facts WHERE active = 1 AND %s
		)
	`, strings.Join(conditions, " AND "))

	s.db.Exec(deleteVecQuery, args...)

	// delete facts
	deleteQuery := fmt.Sprintf(`
		DELETE FROM facts WHERE active = 1 AND %s
	`, strings.Join(conditions, " AND "))

	result, err := s.db.Exec(deleteQuery, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
