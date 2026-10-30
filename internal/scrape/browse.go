package scrape

import (
	"context"
	"strconv"
	"strings"

	"retrove/internal/library"
)

// BrowseMediaResult is the response for media browse UI.
type BrowseMediaResult struct {
	Provider  string       `json:"provider"`
	Kind      string       `json:"kind"`
	GameID    string       `json:"game_id,omitempty"`
	GameLabel string       `json:"game_label,omitempty"`
	Items     []MediaImage `json:"items"`
}

// BrowseMedia lists all remote images of a kind from one provider for a library game.
func (s *Service) BrowseMedia(ctx context.Context, game library.Game, provider, kind, query string) (BrowseMediaResult, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "wheel" {
		kind = KindLogo
	}
	out := BrowseMediaResult{Provider: provider, Kind: kind, Items: []MediaImage{}}
	if !ProviderSupportsKind(provider, kind) {
		return out, classify(KindConfig, "该源不支持此媒体类型", nil)
	}
	q := strings.TrimSpace(query)
	if q == "" {
		q = strings.TrimSpace(game.TitleEN)
	}
	if q == "" {
		return out, classify(KindNotFound, "无可用英文名", nil)
	}

	switch provider {
	case "screenscraper", "ss":
		out.Provider = "screenscraper"
		return s.browseSS(ctx, game, kind, q, out)
	case "steamgriddb", "sgdb":
		out.Provider = "steamgriddb"
		return s.browseSGDB(ctx, game, kind, q, out)
	case "igdb":
		out.Provider = "igdb"
		return s.browseIGDB(ctx, game, kind, q, out)
	default:
		return out, classify(KindConfig, "未知刮削源: "+provider, nil)
	}
}

func (s *Service) browseSS(ctx context.Context, game library.Game, kind, query string, out BrowseMediaResult) (BrowseMediaResult, error) {
	if !s.ss.creds.Configured() {
		return out, classify(KindConfig, "未配置 ScreenScraper", nil)
	}
	var info *GameInfo
	if id, ok := parseProviderID(game.ScrapeID, "screenscraper"); ok {
		got, _, err := s.ss.LookupByID(ctx, strconv.Itoa(id))
		if err == nil {
			info = got
		}
	}
	if info == nil {
		systemID, err := s.platformSSID(game.PlatformID)
		if err != nil {
			return out, err
		}
		items, err := s.ss.Search(ctx, query, systemID)
		if err != nil {
			return out, err
		}
		if len(items) == 0 {
			return out, classify(KindNotFound, "ScreenScraper 未命中", nil)
		}
		info = items[0]
		if info.ID != "" {
			if full, _, err := s.ss.LookupByID(ctx, info.ID); err == nil {
				info = full
			}
		}
	}
	out.GameID = info.ID
	out.GameLabel = firstNonEmpty(info.TitleEN, info.TitleZH, info.ID)
	out.Items = ListMediasForKind(info.Medias, kind)
	return out, nil
}

func (s *Service) browseSGDB(ctx context.Context, game library.Game, kind, query string, out BrowseMediaResult) (BrowseMediaResult, error) {
	if !s.sgdb.Configured() {
		return out, classify(KindConfig, "未配置 SteamGridDB", nil)
	}
	gameID := 0
	gameName := query
	if id, ok := parseProviderID(game.ScrapeID, "steamgriddb"); ok {
		gameID = id
	}
	if gameID <= 0 {
		id, name, err := s.sgdb.ResolveGameID(ctx, query)
		if err != nil {
			return out, err
		}
		gameID, gameName = id, name
	}
	out.GameID = strconv.Itoa(gameID)
	out.GameLabel = gameName
	items, err := s.sgdb.ListAssets(ctx, gameID, kind)
	if err != nil {
		return out, err
	}
	out.Items = items
	return out, nil
}

func (s *Service) browseIGDB(ctx context.Context, game library.Game, kind, query string, out BrowseMediaResult) (BrowseMediaResult, error) {
	if !s.igdb.Configured() {
		return out, classify(KindConfig, "未配置 IGDB", nil)
	}
	gameID := 0
	gameName := query
	if id, ok := parseProviderID(game.ScrapeID, "igdb"); ok {
		gameID = id
	}
	if gameID <= 0 {
		platID, err := s.platformIGDBID(game.PlatformID)
		if err != nil {
			return out, err
		}
		id, name, err := s.igdb.ResolveGameID(ctx, query, platID)
		if err != nil {
			return out, err
		}
		gameID, gameName = id, name
	} else if g, err := s.igdb.GetGame(ctx, gameID); err == nil {
		gameName = g.Name
	}
	out.GameID = strconv.Itoa(gameID)
	out.GameLabel = gameName
	items, err := s.igdb.ListAssets(ctx, gameID, kind)
	if err != nil {
		return out, err
	}
	out.Items = items
	return out, nil
}

// PickMedia downloads one remote image URL into the game's media slot.
func (s *Service) PickMedia(ctx context.Context, game library.Game, kind, mediaURL, provider string) (library.Game, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	mediaURL = strings.TrimSpace(mediaURL)
	provider = strings.ToLower(strings.TrimSpace(provider))
	if mediaURL == "" {
		return library.Game{}, classify(KindConfig, "缺少图片 URL", nil)
	}
	switch kind {
	case KindBox, KindScreenshot, KindTitle, KindLogo, "wheel":
		if kind == "wheel" {
			kind = KindLogo
		}
	default:
		return library.Game{}, classify(KindConfig, "不支持的媒体类型", nil)
	}
	if provider == "ss" {
		provider = "screenscraper"
	} else if provider == "sgdb" {
		provider = "steamgriddb"
	}
	t := target{GameID: game.ID, PlatformID: game.PlatformID}
	if _, err := s.saveRemoteMedia(ctx, t, kind, mediaURL, provider); err != nil {
		return library.Game{}, err
	}
	return library.New(s.db).GetGame(game.ID)
}
