package library

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
)

type Stats struct {
	Platforms          int `json:"platforms"`
	PlatformsWithROMs  int `json:"platforms_with_roms"`
	Games              int `json:"games"`
	ROMs               int `json:"roms"`
	Missing            int `json:"missing"`
	Unidentified       int `json:"unidentified"`
	LockedTitles       int `json:"locked_titles"`
	MissingCovers      int `json:"missing_covers"`
	PendingEN          int `json:"pending_en"`
	MissingDescription int `json:"missing_description"`
}

type MediaAsset struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Source string `json:"source"`
}

type Game struct {
	ID                int64        `json:"id"`
	PlatformID        string       `json:"platform_id"`
	PlatformName      string       `json:"platform_name"`
	TitleZH           string       `json:"title_zh"`
	TitleEN           string       `json:"title_en"`
	Description       string       `json:"description"`
	Developer         string       `json:"developer"`
	ReleaseYear       int          `json:"release_year"`
	TitleZHLocked     bool         `json:"title_zh_locked"`
	DescriptionLocked bool         `json:"description_locked"`
	ScrapeID          string       `json:"scrape_id"`
	ROMPath           string       `json:"rom_path"`
	Filename          string       `json:"filename"`
	Missing           bool         `json:"missing"`
	SHA1              string       `json:"sha1"`
	CoverPath         string       `json:"cover_path"`
	Media             []MediaAsset `json:"media,omitempty"`
}

type Service struct {
	db *sql.DB
}

func New(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) Stats() (Stats, error) {
	var st Stats
	err := s.db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM platforms),
			(SELECT COUNT(*) FROM platforms p WHERE EXISTS (
				SELECT 1 FROM rom_files r WHERE r.platform_id=p.id AND r.missing=0)),
			(SELECT COUNT(*) FROM games),
			(SELECT COUNT(*) FROM rom_files),
			(SELECT COUNT(*) FROM rom_files WHERE missing=1),
			(SELECT COUNT(*) FROM games WHERE TRIM(COALESCE(title_zh,'')) = '') +
			(SELECT COUNT(*) FROM rom_files WHERE game_id IS NULL),
			(SELECT COUNT(*) FROM games WHERE title_zh_locked=1),
			(SELECT COUNT(*) FROM games g
			 JOIN rom_files r ON r.game_id=g.id AND r.missing=0
			 WHERE NOT EXISTS (SELECT 1 FROM media_assets m WHERE m.game_id=g.id AND m.kind='box')),
			(SELECT COUNT(*) FROM games g
			 JOIN rom_files r ON r.game_id=g.id AND r.missing=0
			 WHERE TRIM(g.title_zh) != '' AND TRIM(g.title_en) = ''),
			(SELECT COUNT(*) FROM games WHERE TRIM(COALESCE(description,'')) = '')
	`).Scan(
		&st.Platforms,
		&st.PlatformsWithROMs,
		&st.Games,
		&st.ROMs,
		&st.Missing,
		&st.Unidentified,
		&st.LockedTitles,
		&st.MissingCovers,
		&st.PendingEN,
		&st.MissingDescription,
	)
	return st, err
}

func (s *Service) MissingMediaCount() (int, error) {
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM games g
		JOIN rom_files r ON r.game_id = g.id AND r.missing=0
		WHERE NOT EXISTS (SELECT 1 FROM media_assets m WHERE m.game_id=g.id AND m.kind='box')`).Scan(&n)
	return n, err
}

func (s *Service) ListGames(platformID, q, sort string, limit, offset int) ([]Game, int, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	args := []any{}
	where := []string{"1=1"}
	if platformID != "" {
		where = append(where, "g.platform_id=?")
		args = append(args, platformID)
	}
	if q != "" {
		where = append(where, "(g.title_zh LIKE ? OR g.title_en LIKE ? OR r.filename LIKE ? OR r.path LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like, like)
	}
	whereSQL := strings.Join(where, " AND ")
	countSQL := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM games g
		JOIN platforms p ON p.id = g.platform_id
		LEFT JOIN rom_files r ON r.game_id = g.id
		WHERE %s`, whereSQL)
	var total int
	if err := s.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "p.sort_order, g.title_zh"
	if sort == "updated" {
		order = "g.updated_at DESC, g.id DESC"
	}
	listArgs := append(append([]any{}, args...), limit, offset)
	sqlStr := fmt.Sprintf(`
		SELECT g.id, g.platform_id, p.name, g.title_zh, g.title_en, g.description, g.developer,
			COALESCE(g.release_year, 0), g.title_zh_locked, g.description_locked, g.scrape_id,
			COALESCE(r.path,''), COALESCE(r.filename,''), COALESCE(r.missing,0), COALESCE(r.sha1,''),
			COALESCE((SELECT m.path FROM media_assets m WHERE m.game_id=g.id AND m.kind='box' LIMIT 1),'')
		FROM games g
		JOIN platforms p ON p.id = g.platform_id
		LEFT JOIN rom_files r ON r.game_id = g.id
		WHERE %s
		ORDER BY %s
		LIMIT ? OFFSET ?`, whereSQL, order)
	rows, err := s.db.Query(sqlStr, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Game
	for rows.Next() {
		var g Game
		var locked, missing int
		var descLocked int
		if err := rows.Scan(&g.ID, &g.PlatformID, &g.PlatformName, &g.TitleZH, &g.TitleEN, &g.Description, &g.Developer,
			&g.ReleaseYear, &locked, &descLocked, &g.ScrapeID, &g.ROMPath, &g.Filename, &missing, &g.SHA1, &g.CoverPath); err != nil {
			return nil, 0, err
		}
		g.TitleZHLocked = locked == 1
		g.DescriptionLocked = descLocked == 1
		g.Missing = missing == 1
		out = append(out, g)
	}
	return out, total, rows.Err()
}

func (s *Service) GetGame(id int64) (Game, error) {
	var g Game
	var locked, descLocked, missing int
	err := s.db.QueryRow(`
		SELECT g.id, g.platform_id, p.name, g.title_zh, g.title_en, g.description, g.developer,
			COALESCE(g.release_year, 0), g.title_zh_locked, g.description_locked, g.scrape_id,
			COALESCE(r.path,''), COALESCE(r.filename,''), COALESCE(r.missing,0), COALESCE(r.sha1,''),
			COALESCE((SELECT m.path FROM media_assets m WHERE m.game_id=g.id AND m.kind='box' LIMIT 1),'')
		FROM games g
		JOIN platforms p ON p.id = g.platform_id
		LEFT JOIN rom_files r ON r.game_id = g.id
		WHERE g.id=?`, id).Scan(&g.ID, &g.PlatformID, &g.PlatformName, &g.TitleZH, &g.TitleEN, &g.Description, &g.Developer,
		&g.ReleaseYear, &locked, &descLocked, &g.ScrapeID, &g.ROMPath, &g.Filename, &missing, &g.SHA1, &g.CoverPath)
	if err != nil {
		return Game{}, err
	}
	g.TitleZHLocked = locked == 1
	g.DescriptionLocked = descLocked == 1
	g.Missing = missing == 1
	media, err := s.ListMedia(id)
	if err != nil {
		return Game{}, err
	}
	g.Media = media
	return g, nil
}

func (s *Service) ListMedia(gameID int64) ([]MediaAsset, error) {
	rows, err := s.db.Query(`SELECT kind, path, source FROM media_assets WHERE game_id=? ORDER BY kind`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MediaAsset
	for rows.Next() {
		var m MediaAsset
		if err := rows.Scan(&m.Kind, &m.Path, &m.Source); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if out == nil {
		out = []MediaAsset{}
	}
	return out, rows.Err()
}

type UpdateGameInput struct {
	TitleZH           *string `json:"title_zh"`
	TitleEN           *string `json:"title_en"`
	Description       *string `json:"description"`
	Developer         *string `json:"developer"`
	ReleaseYear       *int    `json:"release_year"`
	TitleZHLocked     *bool   `json:"title_zh_locked"`
	DescriptionLocked *bool   `json:"description_locked"`
	ScrapeID          *string `json:"scrape_id"`
}

func (s *Service) UpdateGame(id int64, in UpdateGameInput) (Game, error) {
	g, err := s.GetGame(id)
	if err != nil {
		return Game{}, err
	}
	if in.TitleZH != nil {
		g.TitleZH = *in.TitleZH
	}
	if in.TitleEN != nil {
		g.TitleEN = *in.TitleEN
	}
	if in.Description != nil {
		g.Description = *in.Description
	}
	if in.Developer != nil {
		g.Developer = *in.Developer
	}
	if in.ReleaseYear != nil {
		g.ReleaseYear = *in.ReleaseYear
	}
	if in.ScrapeID != nil {
		g.ScrapeID = *in.ScrapeID
	}
	locked := 0
	if g.TitleZHLocked {
		locked = 1
	}
	if in.TitleZHLocked != nil {
		if *in.TitleZHLocked {
			locked = 1
		} else {
			locked = 0
		}
	}
	descLocked := 0
	if g.DescriptionLocked {
		descLocked = 1
	}
	if in.DescriptionLocked != nil {
		if *in.DescriptionLocked {
			descLocked = 1
		} else {
			descLocked = 0
		}
	}
	var year any
	if g.ReleaseYear > 0 {
		year = g.ReleaseYear
	}
	_, err = s.db.Exec(`UPDATE games SET title_zh=?, title_en=?, description=?, developer=?, release_year=?, scrape_id=?, title_zh_locked=?, description_locked=?, updated_at=datetime('now') WHERE id=?`,
		g.TitleZH, g.TitleEN, g.Description, g.Developer, year, g.ScrapeID, locked, descLocked, id)
	if err != nil {
		return Game{}, err
	}
	return s.GetGame(id)
}

// RenameROM points the game's ROM row at a new filename after the file on disk has moved.
func (s *Service) RenameROM(gameID int64, oldFilename, newFilename, platformDir string) error {
	oldFilename = filepath.Base(strings.TrimSpace(oldFilename))
	newFilename = filepath.Base(strings.TrimSpace(newFilename))
	if gameID <= 0 || oldFilename == "" || newFilename == "" || oldFilename == newFilename {
		return nil
	}
	newPath := filepath.ToSlash(filepath.Join(platformDir, newFilename))
	res, err := s.db.Exec(`UPDATE rom_files SET path=?, filename=?, updated_at=datetime('now') WHERE game_id=? AND filename=?`,
		newPath, newFilename, gameID, oldFilename)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("未找到要改名的 ROM")
	}
	oldStem := strings.TrimSuffix(oldFilename, filepath.Ext(oldFilename))
	newStem := strings.TrimSuffix(newFilename, filepath.Ext(newFilename))
	_, err = s.db.Exec(`UPDATE aliases SET value=? WHERE game_id=? AND kind='filename' AND value=?`, newStem, gameID, oldStem)
	return err
}
