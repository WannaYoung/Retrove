package platform

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Definition struct {
	ID              string   `yaml:"id" json:"id"`
	Name            string   `yaml:"name" json:"name"`
	Directory       string   `yaml:"directory" json:"directory"`
	Extensions      []string `yaml:"extensions" json:"extensions"`
	HashMode        string   `yaml:"hash_mode" json:"hash_mode"`
	RenamePolicy    string   `yaml:"rename_policy" json:"rename_policy"`
	ScreenScraperID int      `yaml:"screenscraper_id" json:"screenscraper_id"`
	IGDBID          int      `yaml:"igdb_id" json:"igdb_id"`
	ScrapeAs        string   `yaml:"scrape_as" json:"scrape_as"`
	SortOrder       int      `yaml:"sort_order" json:"sort_order"`
	ROMCount        int      `yaml:"-" json:"rom_count,omitempty"`
	PendingEN       int      `yaml:"-" json:"pending_en,omitempty"`
	MissingCovers   int      `yaml:"-" json:"missing_covers,omitempty"`
}

type File struct {
	Platforms []Definition `yaml:"platforms"`
}

func LoadFile(path string) ([]Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	for i := range f.Platforms {
		p := &f.Platforms[i]
		if p.ID == "" || p.Directory == "" {
			return nil, fmt.Errorf("platform #%d missing id/directory", i)
		}
		if p.Name == "" {
			p.Name = p.Directory
		}
		if p.HashMode == "" {
			p.HashMode = "content"
		}
		if p.RenamePolicy == "" {
			p.RenamePolicy = "allow"
		}
		if p.SortOrder == 0 {
			p.SortOrder = (i + 1) * 10
		}
		for j := range p.Extensions {
			p.Extensions[j] = strings.ToLower(strings.TrimPrefix(p.Extensions[j], "."))
		}
	}
	return f.Platforms, nil
}

func Sync(db *sql.DB, defs []Definition) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, p := range defs {
		_, err := tx.Exec(`
			INSERT INTO platforms (id, name, directory, extensions, hash_mode, rename_policy, screenscraper_id, igdb_id, scrape_as, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				name=excluded.name,
				directory=excluded.directory,
				extensions=excluded.extensions,
				hash_mode=excluded.hash_mode,
				rename_policy=excluded.rename_policy,
				screenscraper_id=excluded.screenscraper_id,
				igdb_id=excluded.igdb_id,
				scrape_as=excluded.scrape_as,
				sort_order=excluded.sort_order
		`, p.ID, p.Name, p.Directory, strings.Join(p.Extensions, ","), p.HashMode, p.RenamePolicy, p.ScreenScraperID, p.IGDBID, p.ScrapeAs, p.SortOrder)
		if err != nil {
			return fmt.Errorf("upsert %s: %w", p.ID, err)
		}
	}
	return tx.Commit()
}

func List(db *sql.DB) ([]Definition, error) {
	rows, err := db.Query(`
		SELECT p.id, p.name, p.directory, p.extensions, p.hash_mode, p.rename_policy, p.screenscraper_id, p.igdb_id, p.scrape_as, p.sort_order,
			(SELECT COUNT(*) FROM rom_files r WHERE r.platform_id=p.id AND r.missing=0) AS rom_count,
			(SELECT COUNT(DISTINCT g.id) FROM games g
			 JOIN rom_files r ON r.game_id=g.id AND r.missing=0
			 WHERE g.platform_id=p.id AND TRIM(g.title_zh) != '' AND TRIM(g.title_en) = '') AS pending_en,
			(SELECT COUNT(*) FROM games g
			 JOIN rom_files r ON r.game_id=g.id AND r.missing=0
			 WHERE g.platform_id=p.id
			   AND NOT EXISTS (SELECT 1 FROM media_assets m WHERE m.game_id=g.id AND m.kind='box')) AS missing_covers
		FROM platforms p
		ORDER BY p.sort_order, p.directory`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Definition
	for rows.Next() {
		var p Definition
		var ext string
		if err := rows.Scan(&p.ID, &p.Name, &p.Directory, &ext, &p.HashMode, &p.RenamePolicy, &p.ScreenScraperID, &p.IGDBID, &p.ScrapeAs, &p.SortOrder, &p.ROMCount, &p.PendingEN, &p.MissingCovers); err != nil {
			return nil, err
		}
		if ext != "" {
			p.Extensions = strings.Split(ext, ",")
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func DefaultPath(configDir string) string {
	return filepath.Join(configDir, "platforms.yaml")
}
