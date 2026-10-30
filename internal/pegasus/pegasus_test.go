package pegasus

import (
	"os"
	"path/filepath"
	"testing"

	"retrove/internal/platform"
	"retrove/internal/store"
)

func TestParseAndImport(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "SFC")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `collection: SFC
extensions: sfc, zip

game: 超时空之轮
file: chrono.zip
developer: Square
description: 经典 RPG
`
	if err := os.WriteFile(filepath.Join(dir, "metadata.pegasus.txt"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chrono.zip"), []byte("rom"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := store.Open(filepath.Join(root, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := platform.Sync(db, []platform.Definition{{
		ID: "sfc", Name: "SFC", Directory: "SFC", Extensions: []string{"zip"}, HashMode: "archive", RenamePolicy: "allow", SortOrder: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO games (platform_id, title_zh, title_zh_locked, description_locked, created_at, updated_at) VALUES ('sfc','chrono',1,1,datetime('now'),datetime('now'))`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO rom_files (platform_id, game_id, path, filename, size, mtime, sha1, missing, created_at, updated_at)
		VALUES ('sfc', 1, 'SFC/chrono.zip', 'chrono.zip', 3, 1, 'abc', 0, datetime('now'), datetime('now'))`)
	if err != nil {
		t.Fatal(err)
	}

	col, err := ParseFile(filepath.Join(dir, "metadata.pegasus.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(col.Games) != 1 || col.Games[0].Title != "超时空之轮" {
		t.Fatalf("%+v", col)
	}
	rep, err := ImportAll(db, root)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Updated != 1 {
		t.Fatalf("%+v", rep)
	}
	var title, description string
	var titleLocked, descLocked int
	if err := db.QueryRow(`SELECT title_zh, description, title_zh_locked, description_locked FROM games WHERE id=1`).
		Scan(&title, &description, &titleLocked, &descLocked); err != nil {
		t.Fatal(err)
	}
	if title != "超时空之轮" || description != "经典 RPG" {
		t.Fatalf("title=%q desc=%q", title, description)
	}
	if titleLocked != 1 || descLocked != 1 {
		t.Fatalf("locks should stay on title=%d desc=%d", titleLocked, descLocked)
	}
}
