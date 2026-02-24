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

	// run incremental migrations for existing databases
	if err := s.runMigrations(); err != nil {
		return err
	}

	if err := s.seedDomains(); err != nil {
		return err
	}

	if err := s.seedSheldonEntity(); err != nil {
		return err
	}

	return nil
}

// runMigrations handles schema changes for existing databases
func (s *Store) runMigrations() error {
	// migration: add sensitive column to facts table (added in v0.x)
	if !s.columnExists("facts", "sensitive") {
		if _, err := s.db.Exec("ALTER TABLE facts ADD COLUMN sensitive INTEGER DEFAULT 0"); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) columnExists(table, column string) bool {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == column {
			return true
		}
	}
	return false
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
