package pegasus

import (
	"os"
	"path/filepath"
	"strings"

	"retrove/internal/library"
)

func copyPegasusMedia(db librarySaver, dataRoot, romsRoot, platformDir, platformID string, gameID int64, romFile string) (int, error) {
	if strings.TrimSpace(dataRoot) == "" || gameID <= 0 {
		return 0, nil
	}
	stem := strings.TrimSuffix(filepath.Base(filepath.FromSlash(romFile)), filepath.Ext(romFile))
	if stem == "" {
		return 0, nil
	}
	dir := filepath.Join(romsRoot, platformDir, "media", stem)
	n := 0
	g := library.Game{ID: gameID, PlatformID: platformID}
	for _, item := range []struct{ base, kind string }{
		{"boxFront", "box"},
		{"logo", "logo"},
		{"video", "video"},
	} {
		path := findNamedMedia(dir, item.base, item.kind)
		if path == "" {
			continue
		}
		if _, err := db.SaveMediaFromPath(dataRoot, g, item.kind, path, "pegasus"); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

type librarySaver interface {
	SaveMediaFromPath(dataRoot string, game library.Game, kind, srcPath, source string) (library.MediaAsset, error)
}

func findNamedMedia(dir, base, kind string) string {
	exts := []string{".jpg", ".jpeg", ".png", ".webp", ".gif"}
	if kind == "video" {
		exts = []string{".mp4", ".webm", ".mkv", ".mov"}
	}
	for _, ext := range exts {
		path := filepath.Join(dir, base+ext)
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	allow := map[string]bool{}
	for _, ext := range exts {
		allow[ext] = true
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if strings.EqualFold(strings.TrimSuffix(name, filepath.Ext(name)), base) && allow[ext] {
			return filepath.Join(dir, name)
		}
	}
	return ""
}
