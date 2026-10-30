package scrape

import (
	"path/filepath"
	"testing"

	"retrove/internal/config"
	"retrove/internal/platform"
	"retrove/internal/store"
)

func TestApplyInfoRespectsTitleLock(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := platform.Sync(db, []platform.Definition{{
		ID: "sfc", Name: "SFC", Directory: "SFC", ScreenScraperID: 4, HashMode: "content", RenamePolicy: "allow",
	}}); err != nil {
		t.Fatal(err)
	}
	res, err := db.Exec(`INSERT INTO games (platform_id, title_zh, title_en, title_zh_locked, created_at, updated_at)
		VALUES ('sfc', '锁定中文', '', 1, datetime('now'), datetime('now'))`)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	_, err = db.Exec(`INSERT INTO rom_files (platform_id, game_id, path, filename, size, crc32, md5, sha1, missing, created_at, updated_at)
		VALUES ('sfc', ?, 'SFC/a.zip', 'a.zip', 10, 'AABBCCDD', '', '', 0, datetime('now'), datetime('now'))`, id)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{}
	cfg.Paths.Data = t.TempDir()
	svc := New(db, cfg)
	err = svc.applyInfo(target{
		GameID:        id,
		PlatformID:    "sfc",
		TitleZH:       "锁定中文",
		TitleZHLocked: true,
		Filename:      "a.zip",
	}, &GameInfo{
		ID:      "99",
		TitleEN: "Locked Game",
		TitleZH: "刮削中文",
	}, "ss:99", DefaultFields(), WriteUpdate)
	if err != nil {
		t.Fatal(err)
	}
	var zh, en, scrapeID string
	var locked int
	if err := db.QueryRow(`SELECT title_zh, title_en, scrape_id, title_zh_locked FROM games WHERE id=?`, id).
		Scan(&zh, &en, &scrapeID, &locked); err != nil {
		t.Fatal(err)
	}
	if zh != "锁定中文" || locked != 1 {
		t.Fatalf("lock broken zh=%q locked=%d", zh, locked)
	}
	if en != "Locked Game" || scrapeID != "ss:99" {
		t.Fatalf("en/scrape not applied en=%q scrape=%q", en, scrapeID)
	}
}

func TestApplyInfoRespectsDescriptionLock(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := platform.Sync(db, []platform.Definition{{
		ID: "sfc", Name: "SFC", Directory: "SFC", ScreenScraperID: 4, HashMode: "content", RenamePolicy: "allow",
	}}); err != nil {
		t.Fatal(err)
	}

	res, err := db.Exec(`INSERT INTO games (platform_id, title_zh, title_en, title_zh_locked, description, description_locked, created_at, updated_at)
		VALUES ('sfc', '中文名', 'English', 1, '', 1, datetime('now'), datetime('now'))`)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	info := &GameInfo{ID: "1", TitleEN: "English", TitleZH: "刮削中文", Description: "刮削简介"}
	cfg := config.Config{}
	cfg.Paths.Data = t.TempDir()
	svc := New(db, cfg)
	if err := svc.applyInfo(target{
		GameID:            id,
		PlatformID:        "sfc",
		TitleZH:           "中文名",
		TitleEN:           "English",
		TitleZHLocked:     true,
		DescriptionLocked: true,
		Filename:          "a.zip",
	}, info, "ss:1", DefaultFields(), WriteUpdate); err != nil {
		t.Fatal(err)
	}
	var desc string
	if err := db.QueryRow(`SELECT description FROM games WHERE id=?`, id).Scan(&desc); err != nil {
		t.Fatal(err)
	}
	if desc != "" {
		t.Fatalf("locked empty description should stay empty, got %q", desc)
	}

	if _, err := db.Exec(`UPDATE games SET description_locked=0 WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if err := svc.applyInfo(target{
		GameID:            id,
		PlatformID:        "sfc",
		TitleZH:           "中文名",
		TitleEN:           "English",
		TitleZHLocked:     true,
		DescriptionLocked: false,
		Filename:          "a.zip",
	}, info, "ss:1", DefaultFields(), WriteUpdate); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT description FROM games WHERE id=?`, id).Scan(&desc); err != nil {
		t.Fatal(err)
	}
	if desc != "刮削简介" {
		t.Fatalf("unlocked description not filled: %q", desc)
	}
}

func TestListTargetsRequiresEnglishTitle(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := platform.Sync(db, []platform.Definition{{
		ID: "fc", Name: "FC", Directory: "FC", HashMode: "content", RenamePolicy: "allow",
	}}); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO games (id, platform_id, title_zh, title_en, created_at, updated_at) VALUES
		(1, 'fc', '超级马里奥', '', datetime('now'), datetime('now')),
		(2, 'fc', '魂斗罗', 'Contra', datetime('now'), datetime('now'))`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO rom_files (platform_id, game_id, path, filename, size, missing, created_at, updated_at) VALUES
		('fc', 1, 'FC/a.nes', 'a.nes', 10, 0, datetime('now'), datetime('now')),
		('fc', 2, 'FC/b.nes', 'b.nes', 10, 0, datetime('now'), datetime('now'))`)
	if err != nil {
		t.Fatal(err)
	}

	svc := New(db, config.Config{})
	targets, err := svc.listTargets(Options{Mode: ModePlatform, PlatformID: "fc"})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].GameID != 2 {
		t.Fatalf("expected only Contra, got %#v", targets)
	}
}

func TestScrapeOneSkipsEmptyEnglish(t *testing.T) {
	svc := New(nil, config.Config{})
	updated, media, cached, used, err := svc.scrapeOne(t.Context(), target{GameID: 1, TitleZH: "魂斗罗"}, DefaultFields(), WriteUpdate)
	if err != nil || updated || media != 0 || cached || len(used) != 0 {
		t.Fatalf("updated=%v media=%d cached=%v used=%v err=%v", updated, media, cached, used, err)
	}
}

func TestListTargetsOnlyMissingBox(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := platform.Sync(db, []platform.Definition{{
		ID: "fc", Name: "FC", Directory: "FC", HashMode: "content", RenamePolicy: "allow",
	}}); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO games (id, platform_id, title_zh, title_en, created_at, updated_at) VALUES
		(1, 'fc', 'A', 'A', datetime('now'), datetime('now')),
		(2, 'fc', 'B', 'B', datetime('now'), datetime('now'))`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO rom_files (platform_id, game_id, path, filename, size, missing, created_at, updated_at) VALUES
		('fc', 1, 'FC/a.nes', 'a.nes', 10, 0, datetime('now'), datetime('now')),
		('fc', 2, 'FC/b.nes', 'b.nes', 10, 0, datetime('now'), datetime('now'))`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO media_assets (game_id, kind, path, source) VALUES (1, 'box', 'fc/1/box.png', 'test')`)
	if err != nil {
		t.Fatal(err)
	}

	svc := New(db, config.Config{})
	all, err := svc.listTargets(Options{Mode: ModePlatform, PlatformID: "fc"})
	if err != nil {
		t.Fatal(err)
	}
	miss, err := svc.listTargets(Options{Mode: ModePlatform, PlatformID: "fc", OnlyMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("all=%d", len(all))
	}
	if len(miss) != 1 || miss[0].GameID != 2 {
		t.Fatalf("miss=%#v", miss)
	}
}

func TestNormalizeForcesTitleZHOff(t *testing.T) {
	opts := Options{Mode: ModePlatform, PlatformID: "fc", Fields: Fields{TitleEN: true, TitleZH: true}}
	opts.Normalize()
	if opts.Fields.TitleZH {
		t.Fatal("title_zh should be forced off")
	}
	if opts.WriteMode != WriteUpdate {
		t.Fatalf("default write mode=%q", opts.WriteMode)
	}
}

func TestMergeStrNeverEmptyOverwrite(t *testing.T) {
	if got := mergeStr(WriteOverwrite, "已有", ""); got != "已有" {
		t.Fatalf("overwrite empty: %q", got)
	}
	if got := mergeStr(WriteUpdate, "已有", "新"); got != "已有" {
		t.Fatalf("update keep: %q", got)
	}
	if got := mergeStr(WriteOverwrite, "已有", "新"); got != "新" {
		t.Fatalf("overwrite: %q", got)
	}
	if got := mergeStr(WriteUpdate, "", "新"); got != "新" {
		t.Fatalf("update fill: %q", got)
	}
}
