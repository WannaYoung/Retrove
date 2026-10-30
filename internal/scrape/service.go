package scrape

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"retrove/internal/config"
	"retrove/internal/library"
	"retrove/internal/platform"
)

type Mode string

const (
	ModeMissing  Mode = "missing"
	ModePlatform Mode = "platform"
	ModeOne      Mode = "one"
	ModeBatch    Mode = "batch"
)

// WriteMode controls how scraped values merge with existing data.
type WriteMode string

const (
	WriteUpdate    WriteMode = "update"    // fill missing only; never overwrite with empty
	WriteOverwrite WriteMode = "overwrite" // replace existing with non-empty scraped values
)

// Fields selects which metadata / media kinds to scrape.
// TitleZH is always disabled for batch scrape (Normalize forces it off).
type Fields struct {
	TitleZH     bool `json:"title_zh"`
	TitleEN     bool `json:"title_en"`
	Description bool `json:"description"`
	Developer   bool `json:"developer"`
	Box         bool `json:"box"`
	Screenshot  bool `json:"screenshot"`
	Title       bool `json:"title"`
	Logo        bool `json:"logo"`
	Wheel       bool `json:"wheel"` // legacy alias for Logo
}

func DefaultFields() Fields {
	return Fields{
		TitleZH:     false,
		TitleEN:     true,
		Description: true,
		Developer:   true,
		Box:         true,
		Screenshot:  true,
		Title:       true,
		Logo:        true,
	}
}

func (f Fields) anyMeta() bool {
	return f.TitleZH || f.TitleEN || f.Description || f.Developer
}

func (f Fields) anyMedia() bool {
	return f.Box || f.Screenshot || f.Title || f.Logo || f.Wheel
}

func (f Fields) mediaKinds() []string {
	kinds := make([]string, 0, 4)
	if f.Box {
		kinds = append(kinds, KindBox)
	}
	if f.Screenshot {
		kinds = append(kinds, KindScreenshot)
	}
	if f.Title {
		kinds = append(kinds, KindTitle)
	}
	if f.Logo || f.Wheel {
		kinds = append(kinds, KindLogo)
	}
	return kinds
}

type Options struct {
	Mode        Mode
	PlatformID  string
	GameID      int64
	OnlyMissing bool // missing main cover (box)
	WriteMode   WriteMode
	Fields      Fields
}

// Normalize forces TitleZH off and fills defaults when no field was selected.
func (o *Options) Normalize() {
	if !o.Fields.anyMeta() && !o.Fields.anyMedia() {
		o.Fields = DefaultFields()
	}
	o.Fields.TitleZH = false
	if o.WriteMode != WriteOverwrite {
		o.WriteMode = WriteUpdate
	}
	switch o.Mode {
	case ModeMissing:
		o.OnlyMissing = true
	case ModePlatform:
		// platform-wide scrape; OnlyMissing left as requested
	case ModeBatch:
		if o.PlatformID != "" {
			o.Mode = ModePlatform
		} else if o.OnlyMissing {
			o.Mode = ModeMissing
		} else {
			o.Mode = ModePlatform
		}
	}
}

// mergeStr never replaces existing data with empty scraped values.
func mergeStr(mode WriteMode, current, incoming string) string {
	in := strings.TrimSpace(incoming)
	if in == "" {
		return current
	}
	if mode == WriteOverwrite {
		return in
	}
	if strings.TrimSpace(current) == "" {
		return in
	}
	return current
}

func mergeYear(mode WriteMode, current, incoming int) int {
	if incoming <= 0 {
		return current
	}
	if mode == WriteOverwrite {
		return incoming
	}
	if current <= 0 {
		return incoming
	}
	return current
}

type Failure struct {
	GameID   int64  `json:"game_id"`
	Filename string `json:"filename"`
	Kind     string `json:"kind"`
	Message  string `json:"message"`
}

type Report struct {
	Mode      string         `json:"mode"`
	Attempted int            `json:"attempted"`
	Updated   int            `json:"updated"`
	Media     int            `json:"media"`
	Cached    int            `json:"cached"`
	NotFound  int            `json:"not_found"`
	Skipped   int            `json:"skipped"`
	Sources   map[string]int `json:"sources,omitempty"`
	Failed    []Failure      `json:"failed,omitempty"`
}

type target struct {
	GameID            int64
	PlatformID        string
	TitleZH           string
	TitleEN           string
	Description       string
	Developer         string
	ReleaseYear       int
	TitleZHLocked     bool
	DescriptionLocked bool
	ScrapeID          string
	Filename          string
	ROMPath           string
	CRC32             string
	MD5               string
	SHA1              string
	Size              int64
	Missing           bool
	SSSystemID        int
	HashMode          string
	RenamePolicy      string
	HasBox            bool
}

type Service struct {
	db   *sql.DB
	cfg  config.Config
	ss   *Client
	igdb *IGDBClient
	sgdb *SGDBClient
	http *http.Client
}

func New(db *sql.DB, cfg config.Config) *Service {
	return &Service{
		db:  db,
		cfg: cfg,
		ss: NewClient(Credentials{
			DevID:       cfg.ScreenScraper.DevID,
			DevPassword: cfg.ScreenScraper.DevPassword,
			UserSS:      cfg.ScreenScraper.UserSS,
			UserPass:    cfg.ScreenScraper.UserPass,
			Softname:    cfg.ScreenScraper.Softname,
		}),
		igdb: NewIGDBClient(cfg.IGDB.ClientID, cfg.IGDB.ClientSecret),
		sgdb: NewSGDBClient(cfg.SteamGridDB.APIKey),
		http: &http.Client{Timeout: 45 * time.Second},
	}
}

// TestProvider checks saved credentials for one scrape source.
func (s *Service) TestProvider(ctx context.Context, provider string) error {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "screenscraper", "ss":
		return s.ss.Ping(ctx)
	case "igdb":
		return s.igdb.Ping(ctx)
	case "steamgriddb", "sgdb":
		return s.sgdb.Ping(ctx)
	default:
		return classify(KindConfig, "未知刮削源: "+provider, nil)
	}
}

func (s *Service) Run(ctx context.Context, opts Options, onProgress func(done, total int, msg string)) (Report, error) {
	opts.Normalize()
	if !s.cfg.AnyScrapeSourceConfigured() {
		return Report{}, classify(KindConfig, "未配置任何刮削源（ScreenScraper / IGDB / SteamGridDB）", nil)
	}
	targets, err := s.listTargets(opts)
	if err != nil {
		return Report{}, err
	}
	rep := Report{Mode: string(opts.Mode), Failed: []Failure{}, Sources: map[string]int{}}
	total := len(targets)
	if onProgress != nil {
		onProgress(0, total, fmt.Sprintf("待刮削 %d", total))
	}
	for i, t := range targets {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		if onProgress != nil {
			onProgress(i, total, fmt.Sprintf("刮削 %s", t.Filename))
		}
		rep.Attempted++
		updated, mediaN, cached, used, ferr := s.scrapeOne(ctx, t, opts.Fields, opts.WriteMode)
		for _, src := range used {
			rep.Sources[src]++
		}
		if ferr != nil {
			kind := AsKind(ferr)
			if kind == KindNotFound {
				rep.NotFound++
			} else if kind == KindQuota || kind == KindAuth {
				rep.Failed = append(rep.Failed, Failure{
					GameID: t.GameID, Filename: t.Filename, Kind: string(kind), Message: ferr.Error(),
				})
				if onProgress != nil {
					onProgress(i+1, total, ferr.Error())
				}
				return rep, ferr
			} else {
				rep.Failed = append(rep.Failed, Failure{
					GameID: t.GameID, Filename: t.Filename, Kind: string(kind), Message: ferr.Error(),
				})
			}
			continue
		}
		if cached {
			rep.Cached++
		}
		if updated {
			rep.Updated++
		} else {
			rep.Skipped++
		}
		rep.Media += mediaN
	}
	if onProgress != nil {
		onProgress(total, total, "刮削完成")
	}
	return rep, nil
}

func (s *Service) listTargets(opts Options) ([]target, error) {
	plats, err := platform.List(s.db)
	if err != nil {
		return nil, err
	}
	ssID := map[string]int{}
	hashMode := map[string]string{}
	rename := map[string]string{}
	scrapeAs := map[string]string{}
	for _, p := range plats {
		hashMode[p.ID] = p.HashMode
		rename[p.ID] = p.RenamePolicy
		scrapeAs[p.ID] = p.ScrapeAs
		ssID[p.ID] = p.ScreenScraperID
	}
	for id, as := range scrapeAs {
		if as == "" {
			continue
		}
		if parent, ok := ssID[as]; ok && parent > 0 {
			ssID[id] = parent
		}
	}

	args := []any{}
	where := []string{"r.missing=0", "r.game_id IS NOT NULL", "TRIM(g.title_en) != ''"}
	switch opts.Mode {
	case ModeOne:
		if opts.GameID <= 0 {
			return nil, classify(KindConfig, "scrape_one 需要 game_id", nil)
		}
		where = append(where, "g.id=?")
		args = append(args, opts.GameID)
	case ModePlatform:
		if opts.PlatformID == "" {
			return nil, classify(KindConfig, "scrape_platform 需要 platform_id", nil)
		}
		where = append(where, "g.platform_id=?")
		args = append(args, opts.PlatformID)
		if opts.OnlyMissing {
			where = append(where, `NOT EXISTS (
				SELECT 1 FROM media_assets m WHERE m.game_id=g.id AND m.kind='box'
			)`)
		}
	case ModeMissing:
		where = append(where, `NOT EXISTS (
			SELECT 1 FROM media_assets m WHERE m.game_id=g.id AND m.kind='box'
		)`)
		if opts.PlatformID != "" {
			where = append(where, "g.platform_id=?")
			args = append(args, opts.PlatformID)
		}
	default:
		return nil, classify(KindConfig, "未知刮削模式", nil)
	}

	sqlStr := fmt.Sprintf(`
		SELECT g.id, g.platform_id, g.title_zh, g.title_en, g.description, g.developer,
			COALESCE(g.release_year, 0), g.title_zh_locked, g.description_locked, g.scrape_id,
			COALESCE(r.filename,''), COALESCE(r.path,''), COALESCE(r.crc32,''), COALESCE(r.md5,''), COALESCE(r.sha1,''),
			COALESCE(r.size,0), COALESCE(r.missing,0),
			EXISTS(SELECT 1 FROM media_assets m WHERE m.game_id=g.id AND m.kind='box')
		FROM games g
		JOIN rom_files r ON r.game_id = g.id
		WHERE %s
		ORDER BY g.platform_id, g.id`, strings.Join(where, " AND "))

	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []target
	for rows.Next() {
		var t target
		var locked, descLocked, missing, hasBox int
		if err := rows.Scan(&t.GameID, &t.PlatformID, &t.TitleZH, &t.TitleEN, &t.Description, &t.Developer,
			&t.ReleaseYear, &locked, &descLocked, &t.ScrapeID, &t.Filename, &t.ROMPath, &t.CRC32, &t.MD5, &t.SHA1, &t.Size, &missing, &hasBox); err != nil {
			return nil, err
		}
		t.TitleZHLocked = locked == 1
		t.DescriptionLocked = descLocked == 1
		t.Missing = missing == 1
		t.HasBox = hasBox == 1
		if strings.TrimSpace(t.TitleEN) == "" {
			continue
		}
		t.SSSystemID = ssID[t.PlatformID]
		t.HashMode = hashMode[t.PlatformID]
		t.RenamePolicy = rename[t.PlatformID]
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Service) scrapeOne(ctx context.Context, t target, fields Fields, writeMode WriteMode) (updated bool, mediaN int, cached bool, used []string, err error) {
	if strings.TrimSpace(t.TitleEN) == "" {
		return false, 0, false, nil, nil
	}
	fields.TitleZH = false
	if writeMode != WriteOverwrite {
		writeMode = WriteUpdate
	}
	if !fields.anyMeta() && !fields.anyMedia() {
		return false, 0, false, nil, nil
	}
	hasSS := s.ss.creds.Configured() && t.SSSystemID > 0
	hasIGDB := s.igdb.Configured()
	hasSGDB := s.sgdb.Configured()
	if !hasSS && !hasIGDB && !hasSGDB {
		return false, 0, false, nil, classify(KindConfig, "无可用刮削源", nil)
	}

	cur := t
	var lastErr error
	fatal := func(e error) (bool, int, bool, []string, error) {
		return updated, mediaN, cached, used, e
	}
	markUsed := func(name string) {
		for _, u := range used {
			if u == name {
				return
			}
		}
		used = append(used, name)
	}

	// 1) ScreenScraper — prefer all metadata + media when configured
	if hasSS {
		info, fromCache, _, ssErr := s.lookupSS(ctx, cur)
		if ssErr == nil {
			markUsed("screenscraper")
			cached = fromCache
			if fields.anyMeta() {
				if err := s.applyInfo(cur, info, "ss:"+info.ID, fields, writeMode); err != nil {
					return false, 0, cached, used, err
				}
				updated = true
				if fields.TitleEN {
					cur.TitleEN = mergeStr(writeMode, cur.TitleEN, info.TitleEN)
				}
				if fields.Description {
					cur.Description = mergeStr(writeMode, cur.Description, info.Description)
				}
				if fields.Developer {
					cur.Developer = mergeStr(writeMode, cur.Developer, firstNonEmpty(info.Developer, info.Publisher))
				}
				cur.ScrapeID = "ss:" + info.ID
			}
			if fields.anyMedia() {
				n, dlErr := s.downloadSSMedias(ctx, cur, info, fields, writeMode)
				mediaN += n
				if n > 0 {
					updated = true
				}
				if dlErr != nil {
					kind := AsKind(dlErr)
					if kind == KindQuota || kind == KindAuth {
						return fatal(dlErr)
					}
					lastErr = dlErr
				}
			}
		} else {
			kind := AsKind(ssErr)
			if kind == KindQuota || kind == KindAuth {
				return fatal(ssErr)
			}
			if kind != KindNotFound {
				lastErr = ssErr
			}
		}
	}

	// 2) SteamGridDB — box/title/logo (grids/heroes/logos); keep hero for screenshot fallback
	var sgdbHero string
	needMedia := s.mediaNeeds(cur.GameID, fields, writeMode)
	if hasSGDB && (needMedia[KindBox] || needMedia[KindTitle] || needMedia[KindLogo] || needMedia[KindScreenshot]) {
		q := strings.TrimSpace(cur.TitleEN)
		gameID, gameName, sgErr := s.sgdb.ResolveGameID(ctx, q)
		if sgErr == nil {
			bundle, bErr := s.sgdb.MediaBundleForGame(ctx, gameID, gameName)
			if bErr == nil {
				markUsed("steamgriddb")
				sgdbHero = bundle.Title
				for _, kind := range []string{KindBox, KindTitle, KindLogo} {
					if !needMedia[kind] {
						continue
					}
					url := bundle.URLFor(kind)
					if url == "" {
						continue
					}
					n, dlErr := s.saveRemoteMedia(ctx, cur, kind, url, "steamgriddb")
					if dlErr != nil {
						k := AsKind(dlErr)
						if k == KindQuota || k == KindAuth {
							return fatal(dlErr)
						}
						lastErr = dlErr
						continue
					}
					mediaN += n
					updated = true
				}
			} else if AsKind(bErr) != KindNotFound {
				lastErr = bErr
			}
		} else {
			kind := AsKind(sgErr)
			if kind == KindQuota || kind == KindAuth {
				return fatal(sgErr)
			}
			if kind != KindNotFound {
				lastErr = sgErr
			}
		}
	}

	// 3) IGDB — metadata + SGDB gaps (box/title) + screenshots
	needMeta := fields.anyMeta() && (writeMode == WriteOverwrite ||
		(fields.TitleEN && cur.TitleEN == "") ||
		(fields.Description && cur.Description == "") ||
		(fields.Developer && cur.Developer == ""))
	needMedia = s.mediaNeeds(cur.GameID, fields, writeMode)
	igdbHadScreenshot := false
	if hasIGDB && (needMeta || needMedia[KindBox] || needMedia[KindTitle] || needMedia[KindScreenshot]) {
		q := strings.TrimSpace(cur.TitleEN)
		ig, igErr := s.igdb.SearchGamePrefer(ctx, q, needMedia[KindScreenshot])
		if igErr == nil {
			markUsed("igdb")
			info := &GameInfo{
				ID:          formatIGDBID(ig.ID),
				TitleEN:     ig.Name,
				Description: ig.Summary,
				Developer:   ig.Developer,
				ReleaseYear: ig.ReleaseYear,
			}
			scrapeID := cur.ScrapeID
			if scrapeID == "" || strings.HasPrefix(scrapeID, "igdb:") {
				scrapeID = info.ID
			}
			if fields.anyMeta() && needMeta {
				if err := s.applyInfo(cur, info, scrapeID, fields, writeMode); err != nil {
					return updated, mediaN, cached, used, err
				}
				updated = true
				if fields.TitleEN {
					cur.TitleEN = mergeStr(writeMode, cur.TitleEN, ig.Name)
				}
				if fields.Description {
					cur.Description = mergeStr(writeMode, cur.Description, ig.Summary)
				}
				if fields.Developer {
					cur.Developer = mergeStr(writeMode, cur.Developer, ig.Developer)
				}
				if scrapeID != "" {
					cur.ScrapeID = scrapeID
				}
			}
			// Prefer media already returned with search (avoids 3 extra IGDB calls / rate limits).
			bundle := ig.Bundle()
			if (needMedia[KindBox] && bundle.Box == "") ||
				(needMedia[KindTitle] && bundle.Title == "") ||
				(needMedia[KindScreenshot] && bundle.Screenshot == "") {
				if filled, bErr := s.igdb.MediaBundleForGame(ctx, ig.ID); bErr == nil {
					bundle.MergeFill(filled)
				} else {
					k := AsKind(bErr)
					if k == KindQuota || k == KindAuth {
						return fatal(bErr)
					}
					if k != KindNotFound {
						lastErr = bErr
					}
				}
			}
			if bundle.Screenshot != "" {
				igdbHadScreenshot = true
			}
			for _, kind := range []string{KindBox, KindTitle, KindScreenshot} {
				if !needMedia[kind] {
					continue
				}
				url := bundle.URLFor(kind)
				if url == "" {
					continue
				}
				n, dlErr := s.saveRemoteMedia(ctx, cur, kind, url, "igdb")
				if dlErr != nil {
					k := AsKind(dlErr)
					if k == KindQuota || k == KindAuth {
						return fatal(dlErr)
					}
					lastErr = dlErr
					continue
				}
				mediaN += n
				updated = true
			}
		} else {
			kind := AsKind(igErr)
			if kind == KindQuota || kind == KindAuth {
				return fatal(igErr)
			}
			if kind != KindNotFound {
				lastErr = igErr
			}
		}
	}

	// 4) Screenshot fallback — SGDB hero only when IGDB did not supply a screenshot URL.
	needMedia = s.mediaNeeds(cur.GameID, fields, writeMode)
	if needMedia[KindScreenshot] && sgdbHero != "" && !igdbHadScreenshot {
		n, dlErr := s.saveRemoteMedia(ctx, cur, KindScreenshot, sgdbHero, "steamgriddb")
		if dlErr != nil {
			k := AsKind(dlErr)
			if k == KindQuota || k == KindAuth {
				return fatal(dlErr)
			}
			lastErr = dlErr
		} else {
			mediaN += n
			updated = true
			markUsed("steamgriddb")
		}
	}

	if !updated && mediaN == 0 {
		if lastErr != nil {
			return false, 0, cached, used, lastErr
		}
		return false, 0, cached, used, classify(KindNotFound, "所有已配置源均未命中", nil)
	}
	return updated, mediaN, cached, used, nil
}

func (s *Service) mediaNeeds(gameID int64, fields Fields, writeMode WriteMode) map[string]bool {
	out := map[string]bool{}
	for _, kind := range fields.mediaKinds() {
		if writeMode == WriteOverwrite || !s.hasMedia(gameID, kind) {
			out[kind] = true
		}
	}
	return out
}

func (s *Service) hasBox(gameID int64) bool {
	return s.hasMedia(gameID, "box")
}

func (s *Service) hasMedia(gameID int64, kind string) bool {
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM media_assets WHERE game_id=? AND kind=?`, gameID, kind).Scan(&n)
	return n > 0
}

func (s *Service) lookupSS(ctx context.Context, t target) (*GameInfo, bool, []byte, error) {
	cacheKey := fmt.Sprintf("ss:%d:%s:%s:%s", t.SSSystemID, strings.ToLower(t.SHA1), strings.ToLower(t.MD5), strings.ToLower(t.CRC32))
	if raw, ok, err := getCache(s.db, cacheKey); err != nil {
		return nil, false, nil, err
	} else if ok {
		info, err := parseJeuInfos(raw)
		if err == nil {
			return info, true, raw, nil
		}
	}

	q := LookupQuery{
		SystemID: t.SSSystemID,
		CRC32:    t.CRC32,
		MD5:      t.MD5,
		SHA1:     t.SHA1,
		Filename: t.Filename,
		Size:     t.Size,
		RomType:  "rom",
	}
	info, raw, err := s.ss.Lookup(ctx, q)
	if err != nil && AsKind(err) == KindNotFound && t.Filename != "" {
		q2 := LookupQuery{SystemID: t.SSSystemID, Filename: t.Filename, Size: t.Size, RomType: "rom"}
		info, raw, err = s.ss.Lookup(ctx, q2)
	}
	if err != nil {
		return nil, false, raw, err
	}
	_ = putCache(s.db, cacheKey, "screenscraper", raw)
	return info, false, raw, nil
}

func (s *Service) applyInfo(t target, info *GameInfo, scrapeID string, fields Fields, writeMode WriteMode) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if writeMode != WriteOverwrite {
		writeMode = WriteUpdate
	}

	titleEN := t.TitleEN
	if fields.TitleEN {
		titleEN = mergeStr(writeMode, t.TitleEN, info.TitleEN)
	}
	desc := t.Description
	if fields.Description && !t.DescriptionLocked {
		desc = mergeStr(writeMode, t.Description, info.Description)
	}
	dev := t.Developer
	if fields.Developer {
		dev = mergeStr(writeMode, t.Developer, firstNonEmpty(info.Developer, info.Publisher))
	}

	// Batch scrape never writes Chinese titles.
	titleZH := t.TitleZH
	if fields.TitleZH && !t.TitleZHLocked {
		if zh := s.dictionaryTitle(t.PlatformID, titleEN, t.Filename); zh != "" {
			titleZH = mergeStr(writeMode, t.TitleZH, zh)
		} else {
			titleZH = mergeStr(writeMode, t.TitleZH, info.TitleZH)
		}
	}

	year := t.ReleaseYear
	if info.ReleaseYear > 0 {
		year = mergeYear(writeMode, t.ReleaseYear, info.ReleaseYear)
	}
	var yearVal any
	if year > 0 {
		yearVal = year
	}
	if scrapeID == "" {
		scrapeID = info.ID
	}

	_, err := s.db.Exec(`
		UPDATE games SET
			title_zh=?,
			title_en=?,
			description=?,
			developer=?,
			release_year=?,
			scrape_id=?,
			updated_at=?
		WHERE id=?`,
		titleZH, titleEN, desc, dev, yearVal, scrapeID, now, t.GameID)
	return err
}

func (s *Service) dictionaryTitle(platformID, titleEN, filename string) string {
	candidates := []string{}
	if titleEN != "" {
		candidates = append(candidates, titleEN, strings.ToLower(titleEN))
	}
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	if base != "" {
		candidates = append(candidates, base, strings.ToLower(base))
	}
	for _, c := range candidates {
		var zh string
		err := s.db.QueryRow(`
			SELECT g.title_zh FROM aliases a
			JOIN games g ON g.id = a.game_id
			WHERE a.platform_id=? AND a.value=? AND g.title_zh != ''
			LIMIT 1`, platformID, c).Scan(&zh)
		if err == nil && zh != "" {
			return zh
		}
		err = s.db.QueryRow(`
			SELECT title_zh FROM games
			WHERE platform_id=? AND title_en=? AND title_zh != '' AND title_zh_locked=1
			LIMIT 1`, platformID, c).Scan(&zh)
		if err == nil && zh != "" {
			return zh
		}
	}
	return ""
}

func (s *Service) downloadSSMedias(ctx context.Context, t target, info *GameInfo, fields Fields, writeMode WriteMode) (int, error) {
	kinds := fields.mediaKinds()
	n := 0
	for _, kind := range kinds {
		if writeMode != WriteOverwrite && s.hasMedia(t.GameID, kind) {
			continue
		}
		ref, ok := PreferMedia(info.Medias, kind)
		if !ok {
			continue
		}
		body, ct, err := s.ss.Download(ctx, ref.URL)
		if err != nil {
			return n, err
		}
		if err := s.writeMediaFile(t, kind, body, ct, ref.Format, ref.URL, "screenscraper"); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (s *Service) saveRemoteMedia(ctx context.Context, t target, kind, mediaURL, source string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return 0, classify(KindMedia, "构建媒体下载失败", err)
	}
	req.Header.Set("User-Agent", "Retrove/1.0")
	res, err := s.http.Do(req)
	if err != nil {
		return 0, classify(KindNetwork, "下载媒体失败", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return 0, classify(KindNetwork, "读取媒体失败", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return 0, classify(KindMedia, fmt.Sprintf("媒体 HTTP %d", res.StatusCode), nil)
	}
	if err := s.writeMediaFile(t, kind, body, res.Header.Get("Content-Type"), "", mediaURL, source); err != nil {
		return 0, err
	}
	return 1, nil
}

func (s *Service) writeMediaFile(t target, kind string, body []byte, ct, format, mediaURL, source string) error {
	if kind == "wheel" {
		kind = KindLogo
	}
	ext := extFromContentType(ct, format, mediaURL)
	rel := filepath.ToSlash(filepath.Join(t.PlatformID, fmt.Sprintf("%d", t.GameID), kind+ext))
	abs := filepath.Join(s.cfg.Paths.Data, "media", filepath.FromSlash(rel))
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return classify(KindMedia, "创建媒体目录失败", err)
	}
	if err := os.WriteFile(abs, body, 0o644); err != nil {
		return classify(KindMedia, "写入媒体失败", err)
	}
	library.ClearSiblingMediaFiles(dir, kind, kind+ext)
	return upsertMedia(s.db, t.GameID, kind, rel, source)
}

func extFromContentType(ct, format, mediaURL string) string {
	ct = strings.ToLower(ct)
	switch {
	case strings.Contains(ct, "png"), strings.EqualFold(format, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"), strings.EqualFold(format, "jpg"), strings.EqualFold(format, "jpeg"):
		return ".jpg"
	case strings.Contains(ct, "webp"):
		return ".webp"
	}
	u := strings.ToLower(mediaURL)
	for _, e := range []string{".jpg", ".jpeg", ".png", ".webp"} {
		if strings.Contains(u, e) {
			if e == ".jpeg" {
				return ".jpg"
			}
			return e
		}
	}
	return ".png"
}

func getCache(db *sql.DB, key string) ([]byte, bool, error) {
	var payload string
	err := db.QueryRow(`SELECT payload FROM scrape_cache WHERE key=?`, key).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return []byte(payload), true, nil
}

func putCache(db *sql.DB, key, source string, payload []byte) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO scrape_cache (key, source, payload, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET source=excluded.source, payload=excluded.payload, updated_at=excluded.updated_at`,
		key, source, string(payload), now)
	return err
}

func upsertMedia(db *sql.DB, gameID int64, kind, path, source string) error {
	_, err := db.Exec(`
		INSERT INTO media_assets (game_id, kind, path, source) VALUES (?, ?, ?, ?)
		ON CONFLICT(game_id, kind) DO UPDATE SET path=excluded.path, source=excluded.source`,
		gameID, kind, path, source)
	return err
}

func MediaAbs(dataRoot, rel string) (string, error) {
	rel = filepath.Clean("/" + filepath.ToSlash(rel))
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "..") {
		return "", fmt.Errorf("invalid media path")
	}
	return filepath.Join(dataRoot, "media", filepath.FromSlash(rel)), nil
}

func MustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
