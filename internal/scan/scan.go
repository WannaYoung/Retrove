package scan

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"retrove/internal/hashutil"
	"retrove/internal/platform"
)

type Report struct {
	Added     []string `json:"added"`
	Renamed   []string `json:"renamed"`
	Missing   []string `json:"missing"`
	Duplicate []string `json:"duplicate"`
	Skipped   int      `json:"skipped"`
	Scanned   int      `json:"scanned"`
}

type Options struct {
	RomsRoot       string
	FollowSymlinks bool
}

type ProgressFunc func(done, total int, msg string)

type Service struct {
	db *sql.DB
}

func New(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Run(opts Options, onProgress ProgressFunc) (Report, error) {
	plats, err := platform.List(s.db)
	if err != nil {
		return Report{}, err
	}
	byDir := map[string]platform.Definition{}
	for _, p := range plats {
		byDir[p.Directory] = p
	}

	rep := Report{}
	seenPaths := map[string]struct{}{}
	now := time.Now().UTC().Format(time.RFC3339)

	entries, err := os.ReadDir(opts.RomsRoot)
	if err != nil {
		return Report{}, fmt.Errorf("read roms root: %w", err)
	}

	type pendingFile struct {
		plat platform.Definition
		rel  string
		abs  string
	}
	var files []pendingFile
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		p, ok := byDir[ent.Name()]
		if !ok {
			continue
		}
		dirPath := filepath.Join(opts.RomsRoot, ent.Name())
		err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || strings.EqualFold(name, "metadata.pegasus.txt") {
				return nil
			}
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
			if !allowedExt(p.Extensions, ext) {
				return nil
			}
			rel, err := filepath.Rel(opts.RomsRoot, path)
			if err != nil {
				return err
			}
			files = append(files, pendingFile{plat: p, rel: filepath.ToSlash(rel), abs: path})
			return nil
		})
		if err != nil {
			return rep, err
		}
	}

	total := len(files)
	if onProgress != nil {
		onProgress(0, total, fmt.Sprintf("待扫描 %d", total))
	}
	for i, f := range files {
		seenPaths[f.rel] = struct{}{}
		rep.Scanned++
		if err := s.upsertFile(f.plat, f.rel, f.abs, now, &rep); err != nil {
			return rep, err
		}
		if onProgress != nil && (i%8 == 0 || i+1 == total) {
			onProgress(i+1, total, fmt.Sprintf("扫描 %s", f.rel))
		}
	}

	if onProgress != nil {
		onProgress(total, total, "检查缺失文件")
	}
	rows, err := s.db.Query(`SELECT id, path FROM rom_files WHERE missing=0`)
	if err != nil {
		return rep, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return rep, err
		}
		if _, ok := seenPaths[path]; ok {
			continue
		}
		if _, err := s.db.Exec(`UPDATE rom_files SET missing=1, updated_at=? WHERE id=?`, now, id); err != nil {
			return rep, err
		}
		rep.Missing = append(rep.Missing, path)
	}
	return rep, rows.Err()
}

func (s *Service) upsertFile(p platform.Definition, rel, abs, now string, rep *Report) error {
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	size := info.Size()
	mtime := info.ModTime().Unix()
	filename := filepath.Base(rel)

	var id int64
	var oldPath, sha1, crc32, md5hex string
	var oldSize, oldMtime int64
	var gameID sql.NullInt64
	err = s.db.QueryRow(`SELECT id, path, size, mtime, sha1, crc32, md5, game_id FROM rom_files WHERE path=?`, rel).
		Scan(&id, &oldPath, &oldSize, &oldMtime, &sha1, &crc32, &md5hex, &gameID)
	foundByPath := err == nil
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if foundByPath && sha1 != "" && oldSize == size && oldMtime == mtime {
		rep.Skipped++
		_, err = s.db.Exec(`UPDATE rom_files SET missing=0, updated_at=? WHERE id=?`, now, id)
		return err
	}

	dig, err := hashutil.ForROM(abs, p.HashMode, p.Extensions)
	if err != nil {
		return fmt.Errorf("hash %s: %w", rel, err)
	}

	var hashID int64
	var hashPath string
	err = s.db.QueryRow(`SELECT id, path FROM rom_files WHERE sha1=? AND size=?`, dig.SHA1, dig.Size).
		Scan(&hashID, &hashPath)
	foundByHash := err == nil
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if foundByHash && hashPath != rel {
		if foundByPath && id != hashID {
			if _, err := s.db.Exec(`DELETE FROM rom_files WHERE id=?`, id); err != nil {
				return err
			}
		}
		_, err = s.db.Exec(`UPDATE rom_files SET path=?, filename=?, size=?, mtime=?, crc32=?, md5=?, sha1=?, missing=0, updated_at=? WHERE id=?`,
			rel, filename, dig.Size, mtime, dig.CRC32, dig.MD5, dig.SHA1, now, hashID)
		if err != nil {
			return err
		}
		rep.Renamed = append(rep.Renamed, hashPath+" -> "+rel)
		return nil
	}

	if foundByPath {
		_, err = s.db.Exec(`UPDATE rom_files SET filename=?, size=?, mtime=?, crc32=?, md5=?, sha1=?, missing=0, updated_at=? WHERE id=?`,
			filename, dig.Size, mtime, dig.CRC32, dig.MD5, dig.SHA1, now, id)
		return err
	}

	if foundByHash {
		rep.Duplicate = append(rep.Duplicate, rel+" == "+hashPath)
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	title := strings.TrimSuffix(filename, filepath.Ext(filename))
	gres, err := tx.Exec(`INSERT INTO games (platform_id, title_zh, title_zh_locked, description_locked, created_at, updated_at) VALUES (?, ?, 1, 1, ?, ?)`,
		p.ID, title, now, now)
	if err != nil {
		return err
	}
	newGameID, err := gres.LastInsertId()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO rom_files (platform_id, game_id, path, filename, size, mtime, crc32, md5, sha1, missing, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		p.ID, newGameID, rel, filename, dig.Size, mtime, dig.CRC32, dig.MD5, dig.SHA1, now, now)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	rep.Added = append(rep.Added, rel)
	return nil
}

func allowedExt(exts []string, ext string) bool {
	if len(exts) == 0 {
		return true
	}
	for _, e := range exts {
		if strings.EqualFold(e, ext) {
			return true
		}
	}
	return false
}
