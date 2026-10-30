package scrape

import (
	"context"
	"strconv"
	"strings"

	"retrove/internal/library"
	"retrove/internal/platform"
)

// Candidate is a normalized search/preview result from one scrape source.
type Candidate struct {
	ID          string            `json:"id"`
	Provider    string            `json:"provider"`
	Label       string            `json:"label"`
	TitleZH     string            `json:"title_zh"`
	TitleEN     string            `json:"title_en"`
	Description string            `json:"description"`
	Developer   string            `json:"developer"`
	ReleaseYear int               `json:"release_year,omitempty"`
	Medias      map[string]string `json:"medias"`
}

type ApplySide string

const (
	SideLeft  ApplySide = "left"
	SideRight ApplySide = "right"
)

type ApplyChoices struct {
	TitleZH     ApplySide `json:"title_zh"`
	TitleEN     ApplySide `json:"title_en"`
	Description ApplySide `json:"description"`
	Developer   ApplySide `json:"developer"`
	ReleaseYear ApplySide `json:"release_year"`
	Box         ApplySide `json:"box"`
	Screenshot  ApplySide `json:"screenshot"`
	Title       ApplySide `json:"title"`
	Logo        ApplySide `json:"logo"`
	Wheel       ApplySide `json:"wheel"` // legacy
}

func candidateFromGameInfo(provider, id string, info *GameInfo) Candidate {
	medias := map[string]string{}
	if info == nil {
		return Candidate{ID: id, Provider: provider, Medias: medias}
	}
	for _, kind := range []string{KindBox, KindScreenshot, KindTitle, KindLogo} {
		if ref, ok := PreferMedia(info.Medias, kind); ok && ref.URL != "" {
			medias[kind] = ref.URL
		}
	}
	label := firstNonEmpty(info.TitleEN, info.TitleZH, id)
	return Candidate{
		ID:          id,
		Provider:    provider,
		Label:       label,
		TitleZH:     info.TitleZH,
		TitleEN:     info.TitleEN,
		Description: info.Description,
		Developer:   firstNonEmpty(info.Developer, info.Publisher),
		ReleaseYear: info.ReleaseYear,
		Medias:      medias,
	}
}

func (s *Service) platformSSID(platformID string) (int, error) {
	return s.platformMappedID(platformID, func(p platform.Definition) int { return p.ScreenScraperID })
}

func (s *Service) platformIGDBID(platformID string) (int, error) {
	return s.platformMappedID(platformID, func(p platform.Definition) int { return p.IGDBID })
}

func (s *Service) platformMappedID(platformID string, pick func(platform.Definition) int) (int, error) {
	plats, err := platform.List(s.db)
	if err != nil {
		return 0, err
	}
	ids := map[string]int{}
	scrapeAs := map[string]string{}
	for _, p := range plats {
		ids[p.ID] = pick(p)
		scrapeAs[p.ID] = p.ScrapeAs
	}
	if as := scrapeAs[platformID]; as != "" {
		if parent, ok := ids[as]; ok && parent > 0 {
			return parent, nil
		}
	}
	return ids[platformID], nil
}

// SearchProvider returns candidate summaries for a configured source.
func (s *Service) SearchProvider(ctx context.Context, provider, query, platformID string) ([]Candidate, error) {
	query = strings.TrimSpace(query)
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "screenscraper", "ss":
		return s.searchScreenScraper(ctx, query, platformID)
	case "igdb":
		return s.searchIGDB(ctx, query, platformID)
	case "steamgriddb", "sgdb":
		return s.searchSGDB(ctx, query)
	default:
		return nil, classify(KindConfig, "未知刮削源: "+provider, nil)
	}
}

func (s *Service) searchScreenScraper(ctx context.Context, query, platformID string) ([]Candidate, error) {
	if !s.ss.creds.Configured() {
		return nil, classify(KindConfig, "未配置 ScreenScraper", nil)
	}
	systemID, err := s.platformSSID(platformID)
	if err != nil {
		return nil, err
	}
	items, err := s.ss.Search(ctx, query, systemID)
	if err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(items))
	for _, info := range items {
		c := candidateFromGameInfo("screenscraper", info.ID, info)
		out = append(out, c)
	}
	return out, nil
}

func mediasFromBundle(b MediaBundle) map[string]string {
	medias := map[string]string{}
	if b.Box != "" {
		medias[KindBox] = b.Box
	}
	if b.Screenshot != "" {
		medias[KindScreenshot] = b.Screenshot
	}
	if b.Title != "" {
		medias[KindTitle] = b.Title
	}
	if b.Logo != "" {
		medias[KindLogo] = b.Logo
	}
	return medias
}

func (s *Service) searchIGDB(ctx context.Context, query, platformID string) ([]Candidate, error) {
	if !s.igdb.Configured() {
		return nil, classify(KindConfig, "未配置 IGDB", nil)
	}
	igdbPlat, err := s.platformIGDBID(platformID)
	if err != nil {
		return nil, err
	}
	games, err := s.igdb.SearchGames(ctx, query, igdbPlat, 10)
	if err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(games))
	for _, g := range games {
		out = append(out, Candidate{
			ID:          strconv.Itoa(g.ID),
			Provider:    "igdb",
			Label:       g.Name,
			TitleEN:     g.Name,
			Description: g.Summary,
			Developer:   g.Developer,
			ReleaseYear: g.ReleaseYear,
			Medias:      mediasFromBundle(g.Bundle()),
		})
	}
	return out, nil
}

func (s *Service) searchSGDB(ctx context.Context, query string) ([]Candidate, error) {
	if !s.sgdb.Configured() {
		return nil, classify(KindConfig, "未配置 SteamGridDB", nil)
	}
	items, err := s.sgdb.SearchGames(ctx, query, 10)
	if err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(items))
	for _, it := range items {
		out = append(out, Candidate{
			ID:       strconv.Itoa(it.GameID),
			Provider: "steamgriddb",
			Label:    firstNonEmpty(it.GameName, strconv.Itoa(it.GameID)),
			TitleEN:  it.GameName,
			Medias:   map[string]string{},
		})
	}
	return out, nil
}

// PreviewCandidate loads full details for one search hit.
func (s *Service) PreviewCandidate(ctx context.Context, provider, id, query, platformID string) (Candidate, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	id = strings.TrimSpace(id)
	switch provider {
	case "screenscraper", "ss":
		if !s.ss.creds.Configured() {
			return Candidate{}, classify(KindConfig, "未配置 ScreenScraper", nil)
		}
		info, _, err := s.ss.LookupByID(ctx, id)
		if err != nil {
			return Candidate{}, err
		}
		return candidateFromGameInfo("screenscraper", info.ID, info), nil
	case "igdb":
		igdbID, _ := strconv.Atoi(id)
		if igdbID <= 0 {
			return Candidate{}, classify(KindConfig, "无效 IGDB id", nil)
		}
		g, err := s.igdb.GetGame(ctx, igdbID)
		if err != nil {
			return Candidate{}, err
		}
		bundle := g.Bundle()
		if bundle.Box == "" && bundle.Title == "" && bundle.Screenshot == "" {
			if filled, err := s.igdb.MediaBundleForGame(ctx, igdbID); err == nil {
				bundle = filled
			}
		}
		return Candidate{
			ID:          strconv.Itoa(g.ID),
			Provider:    "igdb",
			Label:       g.Name,
			TitleEN:     g.Name,
			Description: g.Summary,
			Developer:   g.Developer,
			ReleaseYear: g.ReleaseYear,
			Medias:      mediasFromBundle(bundle),
		}, nil
	case "steamgriddb", "sgdb":
		if !s.sgdb.Configured() {
			return Candidate{}, classify(KindConfig, "未配置 SteamGridDB", nil)
		}
		gameID, err := strconv.Atoi(id)
		if err != nil || gameID <= 0 {
			return Candidate{}, classify(KindConfig, "无效 SteamGridDB id", nil)
		}
		name := strings.TrimSpace(query)
		bundle, err := s.sgdb.MediaBundleForGame(ctx, gameID, name)
		if err != nil && AsKind(err) != KindNotFound {
			return Candidate{}, err
		}
		return Candidate{
			ID:       strconv.Itoa(gameID),
			Provider: "steamgriddb",
			Label:    firstNonEmpty(name, id),
			TitleEN:  firstNonEmpty(name),
			Medias:   mediasFromBundle(bundle),
		}, nil
	default:
		return Candidate{}, classify(KindConfig, "未知刮削源: "+provider, nil)
	}
}

// ApplyCandidate merges left/right choices onto the game and downloads selected media.
func (s *Service) ApplyCandidate(ctx context.Context, game library.Game, cand Candidate, choices ApplyChoices) (library.Game, error) {
	lib := library.New(s.db)
	in := library.UpdateGameInput{}

	in.TitleZH = fillStr(game.TitleZH, cand.TitleZH, choices.TitleZH)
	in.TitleEN = fillStr(game.TitleEN, cand.TitleEN, choices.TitleEN)
	in.Description = fillStr(game.Description, cand.Description, choices.Description)
	in.Developer = fillStr(game.Developer, cand.Developer, choices.Developer)
	in.ReleaseYear = fillYear(game.ReleaseYear, cand.ReleaseYear, choices.ReleaseYear)
	if cand.ID != "" {
		scrapeID := cand.Provider + ":" + cand.ID
		if cand.Provider == "screenscraper" {
			scrapeID = "ss:" + cand.ID
		} else if cand.Provider == "igdb" {
			scrapeID = formatIGDBID(mustAtoi(cand.ID))
		}
		in.ScrapeID = &scrapeID
	}

	if _, err := lib.UpdateGame(game.ID, in); err != nil {
		return library.Game{}, err
	}

	t := target{GameID: game.ID, PlatformID: game.PlatformID}
	logoSide := choices.Logo
	if logoSide == "" && choices.Wheel != "" {
		logoSide = choices.Wheel
	}
	mediaSides := map[string]ApplySide{
		KindBox:        choices.Box,
		KindScreenshot: choices.Screenshot,
		KindTitle:      choices.Title,
		KindLogo:       logoSide,
	}
	for kind, side := range mediaSides {
		if side != SideRight || hasMediaKind(game, kind) {
			continue
		}
		url := ""
		if cand.Medias != nil {
			url = cand.Medias[kind]
		}
		if url == "" {
			continue
		}
		if _, err := s.saveRemoteMedia(ctx, t, kind, url, cand.Provider); err != nil {
			return library.Game{}, err
		}
	}
	return lib.GetGame(game.ID)
}

// fillStr applies a scraped value only when the current field is empty.
func fillStr(current, incoming string, side ApplySide) *string {
	if strings.TrimSpace(current) != "" || side != SideRight {
		return nil
	}
	v := incoming
	return &v
}

func fillYear(current, incoming int, side ApplySide) *int {
	if current > 0 || side != SideRight || incoming <= 0 {
		return nil
	}
	v := incoming
	return &v
}

func hasMediaKind(g library.Game, kind string) bool {
	for _, m := range g.Media {
		if m.Kind == kind && strings.TrimSpace(m.Path) != "" {
			return true
		}
	}
	if kind == KindBox && strings.TrimSpace(g.CoverPath) != "" {
		return true
	}
	return false
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// DefaultChoices picks left when both sides have data, otherwise the side that has data.
func DefaultChoices(left library.Game, right Candidate) ApplyChoices {
	leftMedia := map[string]string{}
	for _, m := range left.Media {
		leftMedia[m.Kind] = m.Path
	}
	pick := func(l, r string) ApplySide {
		lh, rh := strings.TrimSpace(l) != "", strings.TrimSpace(r) != ""
		if lh && rh {
			return SideLeft
		}
		if rh && !lh {
			return SideRight
		}
		return SideLeft
	}
	pickYear := func(l, r int) ApplySide {
		lh, rh := l > 0, r > 0
		if lh && rh {
			return SideLeft
		}
		if rh && !lh {
			return SideRight
		}
		return SideLeft
	}
	mediaPick := func(kind string) ApplySide {
		l := leftMedia[kind]
		r := ""
		if right.Medias != nil {
			r = right.Medias[kind]
		}
		return pick(l, r)
	}
	return ApplyChoices{
		TitleZH:     pick(left.TitleZH, right.TitleZH),
		TitleEN:     pick(left.TitleEN, right.TitleEN),
		Description: pick(left.Description, right.Description),
		Developer:   pick(left.Developer, right.Developer),
		ReleaseYear: pickYear(left.ReleaseYear, right.ReleaseYear),
		Box:         mediaPick(KindBox),
		Screenshot:  mediaPick(KindScreenshot),
		Title:       mediaPick(KindTitle),
		Logo:        mediaPick(KindLogo),
	}
}
