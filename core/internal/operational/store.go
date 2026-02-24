package operational

import (
	"database/sql"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// Store provides a shared database for operational (non-memory) data
type Store struct {
	db *sql.DB
}

// Open creates or opens the operational database
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	s := &Store{db: db}
	return s, nil
}

// DB returns the underlying database connection for sub-stores
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the database connection
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
