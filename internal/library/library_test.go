package library

import (
	"database/sql"
	"path/filepath"
	"testing"

	"retrove/internal/platform"
	"retrove/internal/store"
)

func TestStatsAndRecentGames(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "lib.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := platform.Sync(db, []platform.Definition{
		{ID: "fc", Name: "FC", Directory: "FC", Extensions: []string{"nes"}, HashMode: "content", RenamePolicy: "allow", SortOrder: 1},
		{ID: "sfc", Name: "SFC", Directory: "SFC", Extensions: []string{"sfc"}, HashMode: "content", RenamePolicy: "allow", SortOrder: 2},
	}); err != nil {
		t.Fatal(err)
	}

	now := "2026-01-01T00:00:00Z"
	mustExec(t, db, `INSERT INTO games (id, platform_id, title_zh, title_en, description, developer, title_zh_locked, description_locked, scrape_id, created_at, updated_at)
		VALUES (1, 'fc', '超级马里奥', 'Super Mario', '简介', 'Nintendo', 1, 1, '', ?, ?)`, now, now)
	mustExec(t, db, `INSERT INTO games (id, platform_id, title_zh, title_en, description, developer, title_zh_locked, description_locked, scrape_id, created_at, updated_at)
		VALUES (2, 'fc', '魂斗罗', '', '', '', 1, 1, '', ?, ?)`, now, "2026-01-02T00:00:00Z")
	mustExec(t, db, `INSERT INTO games (id, platform_id, title_zh, title_en, description, developer, title_zh_locked, description_locked, scrape_id, created_at, updated_at)
		VALUES (3, 'fc', '', '', '', '', 1, 1, '', ?, ?)`, now, "2026-01-03T00:00:00Z")
	mustExec(t, db, `INSERT INTO rom_files (platform_id, game_id, path, filename, size, mtime, sha1, missing, created_at, updated_at)
		VALUES ('fc', 1, 'FC/mario.nes', 'mario.nes', 10, 1, 'aaa', 0, ?, ?)`, now, now)
	mustExec(t, db, `INSERT INTO rom_files (platform_id, game_id, path, filename, size, mtime, sha1, missing, created_at, updated_at)
		VALUES ('fc', 2, 'FC/contra.nes', 'contra.nes', 11, 1, 'bbb', 0, ?, ?)`, now, now)
	mustExec(t, db, `INSERT INTO rom_files (platform_id, game_id, path, filename, size, mtime, sha1, missing, created_at, updated_at)
		VALUES ('fc', 3, 'FC/unknown.nes', 'unknown.nes', 12, 1, 'ccc', 1, ?, ?)`, now, now)
	mustExec(t, db, `INSERT INTO media_assets (game_id, kind, path, source) VALUES (1, 'box', 'fc/1/box.jpg', 'pegasus')`)

	svc := New(db)
	st, err := svc.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if st.Platforms != 2 || st.PlatformsWithROMs != 1 {
		t.Fatalf("platforms=%d with_roms=%d", st.Platforms, st.PlatformsWithROMs)
	}
	if st.Games != 3 || st.ROMs != 3 {
		t.Fatalf("games=%d roms=%d", st.Games, st.ROMs)
	}
	if st.Missing != 1 || st.Unidentified != 1 || st.PendingEN != 1 || st.MissingCovers != 1 || st.MissingDescription != 2 {
		t.Fatalf("stats %+v", st)
	}

	recent, total, err := svc.ListGames("", "", "updated", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("total=%d", total)
	}
	if len(recent) != 3 || recent[0].ID != 3 || recent[1].ID != 2 {
		t.Fatalf("recent %#v", recent)
	}
	page, pageTotal, err := svc.ListGames("", "", "updated", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if pageTotal != 3 || len(page) != 2 || page[0].ID != 3 {
		t.Fatalf("page %#v total=%d", page, pageTotal)
	}
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatal(err)
	}
}
