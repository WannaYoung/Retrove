package pegasus

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"retrove/internal/library"
	"retrove/internal/platform"
)

type ImportReport struct {
	Platforms int `json:"platforms"`
	Matched   int `json:"matched"`
	Created   int `json:"created"`
	Updated   int `json:"updated"`
	Aliases   int `json:"aliases"`
	Skipped   int `json:"skipped"`
	Media     int `json:"media"`
}

func ImportAll(db *sql.DB, romsRoot, dataRoot string) (ImportReport, error) {
	plats, err := platform.List(db)
	if err != nil {
		return ImportReport{}, err
	}
	rep := ImportReport{}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, p := range plats {
		meta := filepath.Join(romsRoot, p.Directory, "metadata.pegasus.txt")
		if _, err := os.Stat(meta); err != nil {
			continue
		}
		col, err := ParseFile(meta)
		if err != nil {
			return rep, fmt.Errorf("%s: %w", meta, err)
		}
		rep.Platforms++
		for _, g := range col.Games {
			if g.File == "" || g.Title == "" {
				rep.Skipped++
				continue
			}
			rel := filepath.ToSlash(filepath.Join(p.Directory, g.File))
			n, err := applyGame(db, p.ID, p.Directory, romsRoot, dataRoot, rel, g, now)
			if err != nil {
				return rep, err
			}
			rep.Matched += n.Matched
			rep.Created += n.Created
			rep.Updated += n.Updated
			rep.Aliases += n.Aliases
			rep.Media += n.Media
		}
	}
	return rep, nil
}

type applyResult struct {
	Matched, Created, Updated, Aliases, Media int
}

func applyGame(db *sql.DB, platformID, platformDir, romsRoot, dataRoot, rel string, g Game, now string) (applyResult, error) {
	var romID int64
	var gameID sql.NullInt64
	err := db.QueryRow(`SELECT id, game_id FROM rom_files WHERE path=?`, rel).Scan(&romID, &gameID)
	if err == sql.ErrNoRows {
		// create orphan game + placeholder rom path not on disk yet? Skip without rom scan.
		return applyResult{}, nil
	}
	if err != nil {
		return applyResult{}, err
	}
	res := applyResult{Matched: 1}

	title := g.Title
	desc := g.Description
	dev := g.Developer

	if gameID.Valid {
		_, err = db.Exec(`UPDATE games SET
			title_zh=?,
			description=?,
			developer=?,
			updated_at=? WHERE id=?`,
			title, desc, dev, now, gameID.Int64)
		if err != nil {
			return applyResult{}, err
		}
		res.Updated = 1
		n, err := upsertAlias(db, platformID, gameID.Int64, title, filepath.Base(rel))
		if err != nil {
			return applyResult{}, err
		}
		res.Aliases = n
		mediaN, err := copyPegasusMedia(library.New(db), dataRoot, romsRoot, platformDir, platformID, gameID.Int64, g.File)
		if err != nil {
			return applyResult{}, err
		}
		res.Media = mediaN
		return res, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return applyResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	r, err := tx.Exec(`INSERT INTO games (platform_id, title_zh, description, developer, title_zh_locked, description_locked, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, 1, ?, ?)`, platformID, title, desc, dev, now, now)
	if err != nil {
		return applyResult{}, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return applyResult{}, err
	}
	if _, err := tx.Exec(`UPDATE rom_files SET game_id=?, updated_at=? WHERE id=?`, id, now, romID); err != nil {
		return applyResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return applyResult{}, err
	}
	res.Created = 1
	n, err := upsertAlias(db, platformID, id, title, filepath.Base(rel))
	if err != nil {
		return applyResult{}, err
	}
	res.Aliases = n
	mediaN, err := copyPegasusMedia(library.New(db), dataRoot, romsRoot, platformDir, platformID, id, g.File)
	if err != nil {
		return applyResult{}, err
	}
	res.Media = mediaN
	return res, nil
}

func upsertAlias(db *sql.DB, platformID string, gameID int64, title, filename string) (int, error) {
	n := 0
	for _, item := range []struct{ kind, value string }{
		{"name", title},
		{"filename", strings.TrimSuffix(filename, filepath.Ext(filename))},
	} {
		if item.value == "" {
			continue
		}
		_, err := db.Exec(`INSERT INTO aliases (platform_id, game_id, kind, value) VALUES (?, ?, ?, ?)
			ON CONFLICT(platform_id, kind, value) DO UPDATE SET game_id=excluded.game_id`,
			platformID, gameID, item.kind, item.value)
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
