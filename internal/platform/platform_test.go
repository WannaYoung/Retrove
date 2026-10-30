package platform

import (
	"path/filepath"
	"testing"

	"retrove/internal/store"
)

func TestLoadAndSync(t *testing.T) {
	root := filepath.Join("..", "..", "app", "config", "platforms.yaml")
	defs, err := LoadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) < 40 {
		t.Fatalf("got %d platforms", len(defs))
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Sync(db, defs); err != nil {
		t.Fatal(err)
	}
	if err := Sync(db, defs); err != nil {
		t.Fatal(err)
	}
	list, err := List(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(defs) {
		t.Fatalf("%d != %d", len(list), len(defs))
	}
	foundArcade := false
	foundFC := false
	for _, p := range list {
		if p.Directory == "FBNEO STG" {
			foundArcade = true
			if p.HashMode != "archive" || p.RenamePolicy != "deny" {
				t.Fatalf("%+v", p)
			}
			if p.IGDBID != 52 {
				t.Fatalf("arcade igdb_id=%d", p.IGDBID)
			}
		}
		if p.ID == "fc" {
			foundFC = true
			if p.IGDBID != 18 {
				t.Fatalf("fc igdb_id=%d", p.IGDBID)
			}
		}
	}
	if !foundFC {
		t.Fatal("missing FC")
	}
	if !foundArcade {
		t.Fatal("missing FBNEO STG")
	}
}
