package library

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var AllowedMediaKinds = map[string]bool{
	"box":        true,
	"screenshot": true,
	"title":      true,
	"logo":       true,
	"video":      true,
}

func IsVideoKind(kind string) bool {
	return strings.ToLower(strings.TrimSpace(kind)) == "video"
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
	if IsVideoKind(kind) {
		exts = []string{".mp4", ".webm", ".mkv", ".mov", ".avi"}
	}
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
	case ".mp4", ".webm", ".mkv", ".mov":
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
	case strings.Contains(ct, "mp4"), strings.Contains(ct, "mpeg"):
		return ".mp4"
	case strings.Contains(ct, "webm"):
		return ".webm"
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
		if IsVideoKind(kind) {
			ext = ".mp4"
		} else {
			ext = ".png"
		}
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if IsVideoKind(kind) {
		switch ext {
		case ".mp4", ".webm", ".mkv", ".mov":
		default:
			ext = ".mp4"
		}
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

// SaveMediaFromPath copies a local file into the library media slot.
func (s *Service) SaveMediaFromPath(dataRoot string, game Game, kind, srcPath, source string) (MediaAsset, error) {
	kind, err := NormalizeMediaKind(kind)
	if err != nil {
		return MediaAsset{}, err
	}
	srcPath = strings.TrimSpace(srcPath)
	if srcPath == "" {
		return MediaAsset{}, fmt.Errorf("empty media")
	}
	ext := ExtFromFilename(srcPath, "")
	if IsVideoKind(kind) && ext == ".png" {
		ext = ".mp4"
	}
	rel := filepath.ToSlash(filepath.Join(game.PlatformID, fmt.Sprintf("%d", game.ID), kind+ext))
	abs := filepath.Join(dataRoot, "media", filepath.FromSlash(rel))
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return MediaAsset{}, err
	}
	if err := copyFile(srcPath, abs); err != nil {
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

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
