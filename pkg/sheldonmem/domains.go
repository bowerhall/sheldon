package sheldonmem

// DomainSlugToID maps domain slugs to their IDs for quick lookup
var DomainSlugToID = map[string]int{
	"identity":      1,
	"health":        2,
	"mind":          3,
	"beliefs":       4,
	"knowledge":     5,
	"relationships": 6,
	"career":        7,
	"finances":      8,
	"place":         9,
	"goals":         10,
	"preferences":   11,
	"routines":      12,
	"events":        13,
	"patterns":      14,
}

var defaultDomains = []Domain{
	{1, "Identity & Self", "identity", "core"},
	{2, "Body & Health", "health", "core"},
	{3, "Mind & Emotions", "mind", "inner"},
	{4, "Beliefs & Worldview", "beliefs", "inner"},
	{5, "Knowledge & Skills", "knowledge", "inner"},
	{6, "Relationships & Social", "relationships", "world"},
	{7, "Work & Career", "career", "world"},
	{8, "Finances & Assets", "finances", "world"},
	{9, "Place & Environment", "place", "world"},
	{10, "Goals & Aspirations", "goals", "temporal"},
	{11, "Preferences & Tastes", "preferences", "meta"},
	{12, "Rhythms & Routines", "routines", "temporal"},
	{13, "Life Events & Decisions", "events", "temporal"},
	{14, "Unconscious Patterns", "patterns", "meta"},
}

func (s *Store) seedDomains() error {
	for _, d := range defaultDomains {
		_, err := s.db.Exec(queryInsertDomain, d.ID, d.Name, d.Slug, d.Layer)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) GetDomain(id int) (*Domain, error) {
	row := s.db.QueryRow(queryGetDomain, id)
	var d Domain

	err := row.Scan(&d.ID, &d.Name, &d.Slug, &d.Layer)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

func (s *Store) GetDomainBySlug(slug string) (*Domain, error) {
	row := s.db.QueryRow(queryGetDomainBySlug, slug)
	var d Domain

	err := row.Scan(&d.ID, &d.Name, &d.Slug, &d.Layer)
	if err != nil {
		return nil, err
	}

	return &d, nil
}
