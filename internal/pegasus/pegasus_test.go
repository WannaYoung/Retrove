package pegasus

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	rep, err := ImportAll(db, root, filepath.Join(root, "data"))
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
	var developer string
	if err := db.QueryRow(`SELECT developer FROM games WHERE id=1`).Scan(&developer); err != nil {
		t.Fatal(err)
	}
	if developer != "Square" {
		t.Fatalf("developer=%q", developer)
	}
	if titleLocked != 1 || descLocked != 1 {
		t.Fatalf("locks should stay on title=%d desc=%d", titleLocked, descLocked)
	}
}

func TestImportMediaByROMStem(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "FC")
	mediaDir := filepath.Join(dir, "media", "蝙蝠侠2")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `collection: FC

game: 蝙蝠侠2-小丑回归
file: 蝙蝠侠2.zip
developer: 蝙蝠侠2
description: 旧简介
`
	if err := os.WriteFile(filepath.Join(dir, "metadata.pegasus.txt"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "蝙蝠侠2.zip"), []byte("rom"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "boxFront.jpg"), []byte("box-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "logo.png"), []byte("logo-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "video.mp4"), []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := store.Open(filepath.Join(root, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := platform.Sync(db, []platform.Definition{{
		ID: "fc", Name: "FC", Directory: "FC", Extensions: []string{"zip"}, HashMode: "archive", RenamePolicy: "allow", SortOrder: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO games (platform_id, title_zh, description, developer, title_zh_locked, description_locked, created_at, updated_at)
		VALUES ('fc','旧名','库简介','库厂商',1,1,datetime('now'),datetime('now'))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO rom_files (platform_id, game_id, path, filename, size, mtime, sha1, missing, created_at, updated_at)
		VALUES ('fc', 1, 'FC/蝙蝠侠2.zip', '蝙蝠侠2.zip', 3, 1, 'abc', 0, datetime('now'), datetime('now'))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO media_assets (game_id, kind, path, source) VALUES (1, 'box', 'fc/1/box.png', 'old')`); err != nil {
		t.Fatal(err)
	}

	dataRoot := filepath.Join(root, "data")
	rep, err := ImportAll(db, root, dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Updated != 1 || rep.Media != 3 {
		t.Fatalf("%+v", rep)
	}
	var title, desc, dev string
	if err := db.QueryRow(`SELECT title_zh, description, developer FROM games WHERE id=1`).Scan(&title, &desc, &dev); err != nil {
		t.Fatal(err)
	}
	if title != "蝙蝠侠2-小丑回归" || desc != "旧简介" || dev != "蝙蝠侠2" {
		t.Fatalf("title=%q desc=%q dev=%q", title, desc, dev)
	}
	var boxPath, boxSrc, logoSrc string
	if err := db.QueryRow(`SELECT path, source FROM media_assets WHERE game_id=1 AND kind='box'`).Scan(&boxPath, &boxSrc); err != nil {
		t.Fatal(err)
	}
	if boxSrc != "pegasus" || !strings.HasSuffix(boxPath, "box.jpg") {
		t.Fatalf("box path=%q source=%q", boxPath, boxSrc)
	}
	if err := db.QueryRow(`SELECT source FROM media_assets WHERE game_id=1 AND kind='logo'`).Scan(&logoSrc); err != nil {
		t.Fatal(err)
	}
	if logoSrc != "pegasus" {
		t.Fatalf("logo source=%q", logoSrc)
	}
	var videoPath, videoSrc string
	if err := db.QueryRow(`SELECT path, source FROM media_assets WHERE game_id=1 AND kind='video'`).Scan(&videoPath, &videoSrc); err != nil {
		t.Fatal(err)
	}
	if videoSrc != "pegasus" || !strings.HasSuffix(videoPath, "video.mp4") {
		t.Fatalf("video path=%q source=%q", videoPath, videoSrc)
	}
	copiedVideo, err := os.ReadFile(filepath.Join(dataRoot, "media", filepath.FromSlash(videoPath)))
	if err != nil {
		t.Fatal(err)
	}
	if string(copiedVideo) != "video" {
		t.Fatalf("video bytes %q", copiedVideo)
	}
	box, err := os.ReadFile(filepath.Join(dataRoot, "media", filepath.FromSlash(boxPath)))
	if err != nil {
		t.Fatal(err)
	}
	if string(box) != "box-a" {
		t.Fatalf("box bytes %q", box)
	}
	if _, err := os.Stat(filepath.Join(mediaDir, "video.mp4")); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(mediaDir, "boxFront.jpg"), []byte("box-b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportAll(db, root, dataRoot); err != nil {
		t.Fatal(err)
	}
	box, err = os.ReadFile(filepath.Join(dataRoot, "media", filepath.FromSlash(boxPath)))
	if err != nil {
		t.Fatal(err)
	}
	if string(box) != "box-b" {
		t.Fatalf("reimport box %q", box)
	}
}

func TestSyncDetailRenamesROMAndMedia(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "FC")
	mediaDir := filepath.Join(dir, "media", "旧名")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := "collection: FC\nlaunch: keep-me\n\ngame: 别的游戏\nfile: 别的.zip\ndescription: 不动\n\ngame: 旧名\nfile: 旧名.zip\nsort-by: 001\ndeveloper: 旧厂商\ndescription: 旧简介\n"
	if err := os.WriteFile(filepath.Join(dir, "metadata.pegasus.txt"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "旧名.zip"), []byte("rom"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "boxFront.jpg"), []byte("box"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "video.mp4"), []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := SyncDetail(root, "FC", true, "旧名.zip", DetailFields{
		TitleZH: "新名", Description: "新简介", Developer: "新厂商",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Renamed || res.Filename != "新名.zip" {
		t.Fatalf("%+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "新名.zip")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "旧名.zip")); !os.IsNotExist(err) {
		t.Fatalf("old rom still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "media", "新名", "video.mp4")); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "metadata.pegasus.txt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "launch: keep-me") || !strings.Contains(text, "game: 别的游戏") || !strings.Contains(text, "description: 不动") {
		t.Fatalf("unrelated metadata changed:\n%s", text)
	}
	if !strings.Contains(text, "game: 新名") || !strings.Contains(text, "file: 新名.zip") || !strings.Contains(text, "sort-by: 001") || !strings.Contains(text, "description: 新简介") {
		t.Fatalf("block not patched:\n%s", text)
	}

	if err := MirrorMedia(root, "FC", res.Filename, "logo", []byte("logo"), ".png"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "media", "新名", "logo.png")); err != nil {
		t.Fatal(err)
	}
	if err := MirrorMedia(root, "FC", res.Filename, "video", []byte("new-video"), ".mp4"); err != nil {
		t.Fatal(err)
	}
	gotVideo, err := os.ReadFile(filepath.Join(dir, "media", "新名", "video.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotVideo) != "new-video" {
		t.Fatalf("mirrored video %q", gotVideo)
	}
	if _, err := os.Stat(filepath.Join(dir, "media", "新名", "video.mp4")); err != nil {
		t.Fatal(err)
	}

	denied, err := SyncDetail(root, "FC", false, "新名.zip", DetailFields{
		TitleZH: "再改", Description: "只改字段", Developer: "厂商",
	})
	if err != nil {
		t.Fatal(err)
	}
	if denied.Renamed || denied.Filename != "新名.zip" {
		t.Fatalf("deny renamed %+v", denied)
	}
	if _, err := os.Stat(filepath.Join(dir, "新名.zip")); err != nil {
		t.Fatal(err)
	}
}

func TestSyncDetailConflict(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "FC")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "旧名.zip"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "新名.zip"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := SyncDetail(root, "FC", true, "旧名.zip", DetailFields{TitleZH: "新名"})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}
