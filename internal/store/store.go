// Package store owns the write-plane's persistence layer: a SQLite-backed
// table of proxy-issued write tokens. It is the only stateful package in the
// service — token operations attach as methods on *Store rather than
// scattering ad-hoc SQL across handlers.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	// Registers the "sqlite" driver for database/sql. Pure-Go (no cgo), so
	// the binary stays a single static artifact.
	_ "modernc.org/sqlite"
)

// schema creates the tokens table if it does not already exist. It is run
// on every Open call, so it must stay purely additive — never DROP or
// otherwise destructive — to be safe against repeated startups.
const schema = `CREATE TABLE IF NOT EXISTS tokens (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	token TEXT NOT NULL UNIQUE,
	active INTEGER NOT NULL DEFAULT 1,
	created_at INTEGER NOT NULL
)`

// Store is the sole owner of the SQLite handle backing the token table.
// Later token-operation methods (mint, list, toggle, delete) attach here.
type Store struct {
	db *sql.DB
}

// Open opens (creating if necessary) the SQLite database at path,
// guarantees the tokens table exists, and returns a ready-to-use *Store.
//
// The DSN carries busy_timeout and journal_mode pragmas so every pooled
// connection inherits them — pragmas set via a raw PRAGMA statement are
// connection-scoped and would not apply to connections database/sql opens
// later in the pool. Open also forces a real connection via db.Ping, since
// sql.Open itself is lazy and would otherwise defer a bad or locked path to
// the first query instead of surfacing it at startup.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: create db directory %q: %w", dir, err)
		}
	}

	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %q: %w", path, err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping %q: %w", path, err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: create schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database handle.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("store: close: %w", err)
	}
	return nil
}
