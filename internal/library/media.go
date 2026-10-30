package library

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var AllowedMediaKinds = map[string]bool{
	"box":        true,
	"screenshot": true,
	"title":      true,
	"logo":       true,
}

func NormalizeMediaKind(kind string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(kind))
	if k == "wheel" {
		k = "logo" // legacy alias
	}
	if !AllowedMediaKinds[k] {
		return "", fmt.Errorf("unsupported media kind")
	}
	return k, nil
}

// ClearSiblingMediaFiles removes other extensions / legacy wheel names for the same kind slot.
func ClearSiblingMediaFiles(dir, kind, keepName string) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		return
	}
	bases := []string{kind}
	if kind == "logo" {
		bases = append(bases, "wheel")
	}
	exts := []string{".png", ".jpg", ".jpeg", ".webp", ".gif"}
	for _, base := range bases {
		for _, ext := range exts {
			name := base + ext
			if name == keepName {
				continue
			}
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

func ExtFromFilename(name, contentType string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		if ext == ".jpeg" {
			return ".jpg"
		}
		return ext
	}
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	}
	return ".png"
}

func (s *Service) SaveMediaFile(dataRoot string, game Game, kind string, body []byte, ext, source string) (MediaAsset, error) {
	kind, err := NormalizeMediaKind(kind)
	if err != nil {
		return MediaAsset{}, err
	}
	if len(body) == 0 {
		return MediaAsset{}, fmt.Errorf("empty media")
	}
	if ext == "" {
		ext = ".png"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	rel := filepath.ToSlash(filepath.Join(game.PlatformID, fmt.Sprintf("%d", game.ID), kind+ext))
	abs := filepath.Join(dataRoot, "media", filepath.FromSlash(rel))
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return MediaAsset{}, err
	}
	if err := os.WriteFile(abs, body, 0o644); err != nil {
		return MediaAsset{}, err
	}
	ClearSiblingMediaFiles(dir, kind, kind+ext)
	_, err = s.db.Exec(`
		INSERT INTO media_assets (game_id, kind, path, source) VALUES (?, ?, ?, ?)
		ON CONFLICT(game_id, kind) DO UPDATE SET path=excluded.path, source=excluded.source`,
		game.ID, kind, rel, source)
	if err != nil {
		return MediaAsset{}, err
	}
	return MediaAsset{Kind: kind, Path: rel, Source: source}, nil
}

// MigrateWheelToLogo renames legacy wheel media rows/files to logo.
func MigrateWheelToLogo(db *sql.DB, dataRoot string) error {
	rows, err := db.Query(`SELECT id, game_id, path, source FROM media_assets WHERE kind='wheel' OR path LIKE '%/wheel.%'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id     int64
		gameID int64
		path   string
		source string
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.gameID, &r.path, &r.source); err != nil {
			return err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range list {
		oldRel := filepath.ToSlash(r.path)
		newRel := strings.Replace(oldRel, "/wheel.", "/logo.", 1)
		if !strings.Contains(filepath.Base(oldRel), "wheel.") {
			base := filepath.Base(oldRel)
			if strings.HasPrefix(base, "wheel.") {
				newRel = filepath.ToSlash(filepath.Join(filepath.Dir(oldRel), "logo"+filepath.Ext(base)))
			}
		}
		oldAbs := filepath.Join(dataRoot, "media", filepath.FromSlash(oldRel))
		newAbs := filepath.Join(dataRoot, "media", filepath.FromSlash(newRel))
		if oldAbs != newAbs {
			if _, err := os.Stat(oldAbs); err == nil {
				_ = os.MkdirAll(filepath.Dir(newAbs), 0o755)
				if err := os.Rename(oldAbs, newAbs); err != nil {
					// if logo already exists, drop the legacy file
					_ = os.Remove(oldAbs)
				}
			}
		}
		ClearSiblingMediaFiles(filepath.Dir(newAbs), "logo", filepath.Base(newAbs))
		// prefer existing logo row if conflict
		var existing int
		_ = db.QueryRow(`SELECT COUNT(1) FROM media_assets WHERE game_id=? AND kind='logo' AND id!=?`, r.gameID, r.id).Scan(&existing)
		if existing > 0 {
			_, _ = db.Exec(`DELETE FROM media_assets WHERE id=?`, r.id)
			continue
		}
		_, err := db.Exec(`UPDATE media_assets SET kind='logo', path=? WHERE id=?`, newRel, r.id)
		if err != nil {
			return err
		}
	}

	// orphan wheel.* files with no DB row
	mediaRoot := filepath.Join(dataRoot, "media")
	_ = filepath.Walk(mediaRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		base := info.Name()
		if !strings.HasPrefix(base, "wheel.") {
			return nil
		}
		dest := filepath.Join(filepath.Dir(path), "logo"+filepath.Ext(base))
		if _, err := os.Stat(dest); err == nil {
			_ = os.Remove(path)
			return nil
		}
		_ = os.Rename(path, dest)
		return nil
	})

	// drop sibling extensions so only the DB-tracked file remains
	keepRows, err := db.Query(`SELECT kind, path FROM media_assets`)
	if err != nil {
		return err
	}
	defer keepRows.Close()
	for keepRows.Next() {
		var kind, rel string
		if err := keepRows.Scan(&kind, &rel); err != nil {
			return err
		}
		abs := filepath.Join(dataRoot, "media", filepath.FromSlash(rel))
		ClearSiblingMediaFiles(filepath.Dir(abs), kind, filepath.Base(abs))
	}
	return keepRows.Err()
}
