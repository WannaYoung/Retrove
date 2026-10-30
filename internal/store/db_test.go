package store

import (
	"path/filepath"
	"testing"
)

func TestMigrateIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retrove.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var version int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version < 3 {
		t.Fatalf("version %d", version)
	}
	var lockedCol int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('games') WHERE name='description_locked'`).Scan(&lockedCol); err != nil || lockedCol != 1 {
		t.Fatalf("description_locked missing: %v", err)
	}
	tables := []string{"platforms", "games", "rom_files", "aliases", "media_assets", "scrape_cache", "jobs", "users", "sessions"}
	for _, name := range tables {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n); err != nil || n != 1 {
			t.Fatalf("table %s missing: %v", name, err)
		}
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
}
