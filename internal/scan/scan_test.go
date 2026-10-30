package scan

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"retrove/internal/platform"
	"retrove/internal/store"
)

func TestScanRenameKeepsIdentity(t *testing.T) {
	root := t.TempDir()
	roms := filepath.Join(root, "roms")
	sfc := filepath.Join(roms, "SFC")
	if err := os.MkdirAll(sfc, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("fake-sfc-rom-content-aaaaaaaa")
	oldPath := filepath.Join(sfc, "oldgame.sfc")
	if err := os.WriteFile(oldPath, body, 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := store.Open(filepath.Join(root, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := platform.Sync(db, []platform.Definition{{
		ID: "sfc", Name: "SFC", Directory: "SFC", Extensions: []string{"sfc", "zip"}, HashMode: "content", RenamePolicy: "allow", SortOrder: 1,
	}}); err != nil {
		t.Fatal(err)
	}

	svc := New(db)
	rep, err := svc.Run(Options{RomsRoot: roms})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 1 {
		t.Fatalf("added %#v", rep.Added)
	}

	var id int64
	var sha1 string
	if err := db.QueryRow(`SELECT id, sha1 FROM rom_files`).Scan(&id, &sha1); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(sfc, "newgame.sfc")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	rep2, err := svc.Run(Options{RomsRoot: roms})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep2.Renamed) != 1 {
		t.Fatalf("renamed %#v report=%#v", rep2.Renamed, rep2)
	}
	var id2 int64
	var sha12, path string
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM rom_files`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count %d", count)
	}
	if err := db.QueryRow(`SELECT id, sha1, path FROM rom_files`).Scan(&id2, &sha12, &path); err != nil {
		t.Fatal(err)
	}
	if id2 != id || sha12 != sha1 || path != "SFC/newgame.sfc" {
		t.Fatalf("id=%d/%d sha=%s/%s path=%s", id, id2, sha1, sha12, path)
	}
	var titleLocked, descLocked int
	if err := db.QueryRow(`SELECT title_zh_locked, description_locked FROM games`).Scan(&titleLocked, &descLocked); err != nil {
		t.Fatal(err)
	}
	if titleLocked != 1 || descLocked != 1 {
		t.Fatalf("new games should lock title and description, got title=%d desc=%d", titleLocked, descLocked)
	}
}

func TestZipContentHash(t *testing.T) {
	root := t.TempDir()
	roms := filepath.Join(root, "roms", "FC")
	if err := os.MkdirAll(roms, 0o755); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(roms, "demo.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, err := zw.Create("demo.nes")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("NES-ROM-BYTES-12345678")
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = zf.Close()

	db, err := store.Open(filepath.Join(root, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := platform.Sync(db, []platform.Definition{{
		ID: "fc", Name: "FC", Directory: "FC", Extensions: []string{"nes", "zip"}, HashMode: "content", RenamePolicy: "allow", SortOrder: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := New(db).Run(Options{RomsRoot: filepath.Join(root, "roms")}); err != nil {
		t.Fatal(err)
	}
	var sha1 string
	if err := db.QueryRow(`SELECT sha1 FROM rom_files`).Scan(&sha1); err != nil {
		t.Fatal(err)
	}
	if sha1 == "" {
		t.Fatal("empty sha1")
	}
}
