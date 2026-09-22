// Command querysqlite is a tiny helper for the live E2E script: it runs a
// SELECT against a SQLite file and prints the rows as JSON. It exists so the
// script can inspect the database without depending on a SQLite CLI.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := flag.String("db", "", "path to sqlite database")
	query := flag.String("query", "", "SELECT query to run")
	flag.Parse()
	if *dbPath == "" || *query == "" {
		fmt.Fprintln(os.Stderr, "usage: querysqlite -db FILE -query SQL")
		os.Exit(2)
	}
	d, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		fatal(err)
	}
	defer d.Close()
	rows, err := d.Query(*query)
	if err != nil {
		fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		fatal(err)
	}
	out := make([]map[string]any, 0)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			fatal(err)
		}
		m := map[string]any{}
		for i, c := range cols {
			m[c] = vals[i]
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		fatal(err)
	}
	// Compact single-line JSON: the PowerShell wrapper pipes stdout to
	// ConvertFrom-Json, which splits pretty-printed multi-line output into one
	// broken object per line.
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "querysqlite:", err)
	os.Exit(1)
}
