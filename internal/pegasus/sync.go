package pegasus

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ConflictError means a ROM file or Pegasus media folder already occupies the target name.
type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string {
	if e == nil {
		return "目标已存在"
	}
	return e.Message
}

// DetailFields are the Pegasus-backed values written back from the game detail page.
type DetailFields struct {
	TitleZH     string
	Description string
	Developer   string
}

// SyncResult reports the ROM filename after a detail-page sync.
type SyncResult struct {
	Filename string
	Renamed  bool
}

// SyncDetail updates metadata.pegasus.txt for one game. When rename is allowed and the
// Chinese title changes, it also renames the ROM file and the media/{stem} folder.
func SyncDetail(romsRoot, platformDir string, allowRename bool, oldFilename string, fields DetailFields) (SyncResult, error) {
	oldFilename = filepath.Base(strings.TrimSpace(oldFilename))
	res := SyncResult{Filename: oldFilename}
	if oldFilename == "" || oldFilename == "." {
		return res, nil
	}

	newFilename := oldFilename
	if allowRename && strings.TrimSpace(fields.TitleZH) != "" {
		renamed, err := renameROMAndMedia(romsRoot, platformDir, oldFilename, fields.TitleZH)
		if err != nil {
			return res, err
		}
		if renamed != "" {
			newFilename = renamed
			res.Filename = renamed
			res.Renamed = true
		}
	}

	meta := Game{
		Title:       oneLine(fields.TitleZH),
		File:        newFilename,
		Developer:   oneLine(fields.Developer),
		Description: oneLine(fields.Description),
	}
	if err := patchMetadata(filepath.Join(romsRoot, platformDir, "metadata.pegasus.txt"), platformDir, oldFilename, meta); err != nil {
		if newFilename != oldFilename {
			rollbackRename(romsRoot, platformDir, oldFilename, newFilename)
		}
		return SyncResult{}, err
	}
	return res, nil
}

func rollbackRename(romsRoot, platformDir, oldFilename, newFilename string) {
	newROM := filepath.Join(romsRoot, platformDir, newFilename)
	oldROM := filepath.Join(romsRoot, platformDir, oldFilename)
	if _, err := os.Stat(oldROM); os.IsNotExist(err) {
		_ = os.Rename(newROM, oldROM)
	}
	oldStem := strings.TrimSuffix(oldFilename, filepath.Ext(oldFilename))
	newStem := strings.TrimSuffix(newFilename, filepath.Ext(newFilename))
	if oldStem == newStem {
		return
	}
	newMedia := filepath.Join(romsRoot, platformDir, "media", newStem)
	oldMedia := filepath.Join(romsRoot, platformDir, "media", oldStem)
	if _, err := os.Stat(oldMedia); os.IsNotExist(err) {
		if st, err := os.Stat(newMedia); err == nil && st.IsDir() {
			_ = os.Rename(newMedia, oldMedia)
		}
	}
}

// MirrorMedia writes a library box, logo, or preview video into the Pegasus media folder.
func MirrorMedia(romsRoot, platformDir, romFilename, kind string, body []byte, ext string) error {
	kind = strings.ToLower(strings.TrimSpace(kind))
	base := ""
	switch kind {
	case "box":
		base = "boxFront"
	case "logo":
		base = "logo"
	case "video":
		base = "video"
	default:
		return nil
	}
	romFilename = filepath.Base(strings.TrimSpace(romFilename))
	if romFilename == "" || romFilename == "." || len(body) == 0 {
		return nil
	}
	ext = strings.ToLower(ext)
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	imageExts := []string{".png", ".jpg", ".jpeg", ".webp", ".gif"}
	videoExts := []string{".mp4", ".webm", ".mkv", ".mov"}
	allowed := imageExts
	if kind == "video" {
		allowed = videoExts
		if !extAllowed(ext, allowed) {
			ext = ".mp4"
		}
	} else if !extAllowed(ext, allowed) {
		ext = ".png"
	}
	stem := strings.TrimSuffix(romFilename, filepath.Ext(romFilename))
	dir := filepath.Join(romsRoot, platformDir, "media", stem)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	keep := base + ext
	for _, e := range allowed {
		name := base + e
		if name == keep {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
	return os.WriteFile(filepath.Join(dir, keep), body, 0o644)
}

func extAllowed(ext string, allowed []string) bool {
	for _, e := range allowed {
		if e == ext {
			return true
		}
	}
	return false
}

func renameROMAndMedia(romsRoot, platformDir, oldFilename, title string) (string, error) {
	stem, err := ROMStem(title)
	if err != nil {
		return "", err
	}
	newFilename := stem + filepath.Ext(oldFilename)
	if newFilename == oldFilename {
		return "", nil
	}
	oldROM := filepath.Join(romsRoot, platformDir, oldFilename)
	newROM := filepath.Join(romsRoot, platformDir, newFilename)
	st, err := os.Stat(oldROM)
	if err != nil || st.IsDir() {
		return "", nil
	}
	if _, err := os.Stat(newROM); err == nil {
		return "", &ConflictError{Message: "目标 ROM 已存在"}
	}
	oldMedia := filepath.Join(romsRoot, platformDir, "media", strings.TrimSuffix(oldFilename, filepath.Ext(oldFilename)))
	newMedia := filepath.Join(romsRoot, platformDir, "media", stem)
	if oldMedia != newMedia {
		if _, err := os.Stat(newMedia); err == nil {
			return "", &ConflictError{Message: "目标媒体目录已存在"}
		}
	}
	if err := os.Rename(oldROM, newROM); err != nil {
		return "", err
	}
	if info, err := os.Stat(oldMedia); err == nil && info.IsDir() && oldMedia != newMedia {
		if err := os.MkdirAll(filepath.Dir(newMedia), 0o755); err != nil {
			_ = os.Rename(newROM, oldROM)
			return "", err
		}
		if err := os.Rename(oldMedia, newMedia); err != nil {
			_ = os.Rename(newROM, oldROM)
			return "", err
		}
	}
	return newFilename, nil
}

// ROMStem builds a single path segment from a Chinese title.
func ROMStem(title string) (string, error) {
	title = strings.TrimSpace(title)
	title = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return -1
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, title)
	title = strings.TrimSpace(title)
	title = strings.TrimRight(title, ". ")
	if title == "" || title == "." || title == ".." {
		return "", fmt.Errorf("无效的 ROM 文件名")
	}
	if len(title) > 240 {
		return "", fmt.Errorf("ROM 文件名过长")
	}
	if !utf8.ValidString(title) {
		return "", fmt.Errorf("无效的 ROM 文件名")
	}
	return title, nil
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.TrimSpace(s)
}

func patchMetadata(path, collection, matchFile string, g Game) error {
	matchFile = filepath.Base(strings.TrimSpace(matchFile))
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	nl := "\n"
	if strings.Contains(string(raw), "\r\n") {
		nl = "\r\n"
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	var lines []string
	if text != "" {
		lines = strings.Split(text, "\n")
	}

	g.File = replaceMetaFilename(metaFileIn(lines, matchFile), g.File)
	found := false
	out := make([]string, 0, len(lines)+6)
	for i := 0; i < len(lines); {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "game:") {
			out = append(out, lines[i])
			i++
			continue
		}
		j := i + 1
		for j < len(lines) {
			next := strings.TrimSpace(lines[j])
			if next == "" || strings.HasPrefix(next, "game:") {
				break
			}
			j++
		}
		block := append([]string(nil), lines[i:j]...)
		if fileMatches(blockFile(block), matchFile) {
			block = rewriteBlock(block, g)
			found = true
		}
		out = append(out, block...)
		i = j
	}
	if !found {
		if len(out) == 0 {
			name := strings.TrimSpace(collection)
			if name == "" {
				name = "collection"
			}
			out = append(out, "collection: "+name, "")
		} else if strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		out = append(out,
			"game: "+g.Title,
			"file: "+g.File,
			"developer: "+g.Developer,
			"description: "+g.Description,
			"",
		)
	}
	body := strings.Join(out, nl)
	if !strings.HasSuffix(body, nl) {
		body += nl
	}
	return writeAtomic(path, []byte(body))
}

func metaFileIn(lines []string, matchFile string) string {
	for i := 0; i < len(lines); {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "game:") {
			i++
			continue
		}
		j := i + 1
		for j < len(lines) {
			next := strings.TrimSpace(lines[j])
			if next == "" || strings.HasPrefix(next, "game:") {
				break
			}
			j++
		}
		block := lines[i:j]
		if fileMatches(blockFile(block), matchFile) {
			return blockFile(block)
		}
		i = j
	}
	return matchFile
}

func replaceMetaFilename(oldMeta, newFilename string) string {
	oldMeta = strings.TrimSpace(oldMeta)
	newFilename = filepath.Base(strings.TrimSpace(newFilename))
	if oldMeta == "" || newFilename == "" {
		return newFilename
	}
	dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(oldMeta)))
	if dir == "." || dir == "" {
		return newFilename
	}
	return dir + "/" + newFilename
}

func blockFile(block []string) string {
	for _, line := range block {
		key, val, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && strings.TrimSpace(key) == "file" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

func fileMatches(metaFile, filename string) bool {
	metaFile = strings.TrimSpace(metaFile)
	filename = filepath.Base(strings.TrimSpace(filename))
	if metaFile == "" || filename == "" {
		return false
	}
	if metaFile == filename {
		return true
	}
	return filepath.Base(filepath.FromSlash(metaFile)) == filename
}

func rewriteBlock(block []string, g Game) []string {
	out := make([]string, 0, len(block)+3)
	hasDev, hasDesc := false, false
	for _, line := range block {
		key, _, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			out = append(out, line)
			continue
		}
		switch strings.TrimSpace(key) {
		case "game":
			if g.Title != "" {
				out = append(out, "game: "+g.Title)
			} else {
				out = append(out, line)
			}
		case "file":
			out = append(out, "file: "+g.File)
		case "developer":
			out = append(out, "developer: "+g.Developer)
			hasDev = true
		case "description":
			out = append(out, "description: "+g.Description)
			hasDesc = true
		default:
			out = append(out, line)
		}
	}
	if !hasDev {
		out = append(out, "developer: "+g.Developer)
	}
	if !hasDesc {
		out = append(out, "description: "+g.Description)
	}
	return out
}

func writeAtomic(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".metadata-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
