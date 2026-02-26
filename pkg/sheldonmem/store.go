package sheldonmem

import (
	"database/sql"

	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
	_ "github.com/ncruces/go-sqlite3/driver"
)

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	if _, err := s.db.Exec(vecSchema); err != nil {
		return err
	}

	// Add tier column to existing notes tables (migration for existing DBs)
	// Ignore error if column already exists
	s.db.Exec("ALTER TABLE notes ADD COLUMN tier TEXT DEFAULT 'working'")
	// Create index after ensuring column exists
	s.db.Exec("CREATE INDEX IF NOT EXISTS idx_notes_tier ON notes(tier)")

	if err := s.seedDomains(); err != nil {
		return err
	}

	if err := s.seedSheldonEntity(); err != nil {
		return err
	}

	return nil
}

func (s *Store) SetEmbedder(e Embedder) {
	s.embedder = e
}

func (s *Store) HasEmbedder() bool {
	return s.embedder != nil
}

func (s *Store) seedSheldonEntity() error {
	var count int

	err := s.db.QueryRow(queryCountSheldonEntity).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		_, err = s.db.Exec(queryInsertEntity, "Sheldon", "agent", 1, `{"role":"assistant"}`)

		return err
	}

	return nil
}

func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}

	return nil
}

func (s *Store) DB() *sql.DB {
	return s.db
}
