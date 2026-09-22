package db

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies all embedded SQL migrations in order.
// Migrations are idempotent (CREATE ... IF NOT EXISTS), which keeps the runner
// simple and re-runnable — no separate schema_migrations bookkeeping needed yet.
func Migrate(d *sqlx.DB) error {
	// embed.FS always uses slash-separated paths, even on Windows.
	names, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	files := make([]string, 0, len(names))
	for _, f := range names {
		if strings.HasSuffix(f.Name(), ".sql") {
			files = append(files, f.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		stmt, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := d.Exec(string(stmt)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}
