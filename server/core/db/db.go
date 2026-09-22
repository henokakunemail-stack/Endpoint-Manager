package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo
)

// Open creates/opens the SQLite database and applies migrations.
// Uses WAL mode + busy_timeout for concurrent reader/writer access.
func Open(dbPath string) (*sqlx.DB, error) {
	// Make sure the data directory exists (e.g. ./data).
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir %s: %w", dir, err)
		}
	}

	d, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}

	// Single writer avoids SQLITE_BUSY under concurrency; busy_timeout covers
	// the brief windows between releases.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := d.Exec(p); err != nil {
			d.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	if err := d.Ping(); err != nil {
		d.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if err := Migrate(d); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}
