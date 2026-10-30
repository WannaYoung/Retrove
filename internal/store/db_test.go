package store

import (
	"path/filepath"
	"testing"
)

func TestEnsureSchemaIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retrove.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var migrations int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='schema_migrations'`).Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if migrations != 0 {
		t.Fatal("schema_migrations should not exist")
	}
	checks := []struct{ table, col string }{
		{"games", "description_locked"},
		{"games", "title_zh_locked"},
		{"platforms", "igdb_id"},
	}
	for _, c := range checks {
		var n int
		q := `SELECT COUNT(*) FROM pragma_table_info('` + c.table + `') WHERE name=?`
		if err := db.QueryRow(q, c.col).Scan(&n); err != nil || n != 1 {
			t.Fatalf("%s.%s missing: %v n=%d", c.table, c.col, err, n)
		}
	}
	tables := []string{"platforms", "games", "rom_files", "aliases", "media_assets", "scrape_cache", "jobs", "users", "sessions"}
	for _, name := range tables {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n); err != nil || n != 1 {
			t.Fatalf("table %s missing: %v", name, err)
		}
	}
	if err := EnsureSchema(db); err != nil {
		t.Fatal(err)
	}
}
