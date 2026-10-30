package scrape

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SGDBClient struct {
	apiKey     string
	httpClient *http.Client
}

func NewSGDBClient(apiKey string) *SGDBClient {
	return &SGDBClient{
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *SGDBClient) Configured() bool {
	return c != nil && c.apiKey != ""
}

type SGDBCover struct {
	GameID   int
	GameName string
	URL      string
}

// Ping verifies the API key with a lightweight autocomplete request.
func (c *SGDBClient) Ping(ctx context.Context) error {
	if !c.Configured() {
		return classify(KindConfig, "未配置 SteamGridDB", nil)
	}
	_, err := c.get(ctx, "https://www.steamgriddb.com/api/v2/search/autocomplete/a")
	return err
}

func (c *SGDBClient) FindCover(ctx context.Context, name string) (*SGDBCover, error) {
	items, err := c.Search(ctx, name, 1)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, classify(KindNotFound, "SteamGridDB 未命中", nil)
	}
	return c.CoverForGame(ctx, items[0].GameID, items[0].GameName)
}

func (c *SGDBClient) Search(ctx context.Context, name string, limit int) ([]SGDBCover, error) {
	if !c.Configured() {
		return nil, classify(KindConfig, "未配置 SteamGridDB", nil)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, classify(KindNotFound, "无可用英文名搜索 SteamGridDB", nil)
	}
	if limit <= 0 {
		limit = 10
	}
	searchURL := "https://www.steamgriddb.com/api/v2/search/autocomplete/" + url.PathEscape(name)
	raw, err := c.get(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	var search struct {
		Success bool `json:"success"`
		Data    []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &search); err != nil {
		return nil, classify(KindParse, "解析 SteamGridDB 搜索失败", err)
	}
	if !search.Success || len(search.Data) == 0 {
		return nil, classify(KindNotFound, "SteamGridDB 未命中", nil)
	}
	if len(search.Data) > limit {
		search.Data = search.Data[:limit]
	}
	out := make([]SGDBCover, 0, len(search.Data))
	for _, game := range search.Data {
		cover, err := c.CoverForGame(ctx, game.ID, game.Name)
		if err != nil {
			out = append(out, SGDBCover{GameID: game.ID, GameName: game.Name})
			continue
		}
		out = append(out, *cover)
	}
	return out, nil
}

func (c *SGDBClient) SearchGames(ctx context.Context, name string, limit int) ([]SGDBCover, error) {
	if !c.Configured() {
		return nil, classify(KindConfig, "未配置 SteamGridDB", nil)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, classify(KindNotFound, "无可用英文名搜索 SteamGridDB", nil)
	}
	if limit <= 0 {
		limit = 10
	}
	searchURL := "https://www.steamgriddb.com/api/v2/search/autocomplete/" + url.PathEscape(name)
	raw, err := c.get(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	var search struct {
		Success bool `json:"success"`
		Data    []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &search); err != nil {
		return nil, classify(KindParse, "解析 SteamGridDB 搜索失败", err)
	}
	if !search.Success || len(search.Data) == 0 {
		return nil, classify(KindNotFound, "SteamGridDB 未命中", nil)
	}
	if len(search.Data) > limit {
		search.Data = search.Data[:limit]
	}
	out := make([]SGDBCover, 0, len(search.Data))
	for _, game := range search.Data {
		out = append(out, SGDBCover{GameID: game.ID, GameName: game.Name})
	}
	return out, nil
}

func (c *SGDBClient) CoverForGame(ctx context.Context, gameID int, gameName string) (*SGDBCover, error) {
	bundle, err := c.MediaBundleForGame(ctx, gameID, gameName)
	if err != nil {
		return nil, err
	}
	if bundle.Box == "" {
		return nil, classify(KindNotFound, "SteamGridDB 无封面", nil)
	}
	return &SGDBCover{GameID: gameID, GameName: gameName, URL: bundle.Box}, nil
}

// MediaBundleForGame maps SGDB assets: grids→box, heroes→title, logos→logo (first image each).
func (c *SGDBClient) MediaBundleForGame(ctx context.Context, gameID int, gameName string) (MediaBundle, error) {
	var out MediaBundle
	if grids, err := c.ListAssets(ctx, gameID, "grids"); err == nil {
		out.Box = PreferJPGURL(grids)
	}
	if heroes, err := c.ListAssets(ctx, gameID, "heroes"); err == nil {
		out.Title = PreferJPGURL(heroes)
	}
	if logos, err := c.ListAssets(ctx, gameID, "logos"); err == nil {
		out.Logo = PreferJPGURL(logos)
	}
	if out.Box == "" && out.Title == "" && out.Logo == "" {
		return out, classify(KindNotFound, "SteamGridDB 无可用媒体", nil)
	}
	return out, nil
}

// ListAssets returns grids / heroes / logos for a SteamGridDB game.
func (c *SGDBClient) ListAssets(ctx context.Context, gameID int, assetType string) ([]MediaImage, error) {
	if !c.Configured() {
		return nil, classify(KindConfig, "未配置 SteamGridDB", nil)
	}
	if gameID <= 0 {
		return nil, classify(KindConfig, "无效 SteamGridDB id", nil)
	}
	assetType = strings.ToLower(strings.TrimSpace(assetType))
	var path string
	switch assetType {
	case "grids", "grid", KindBox:
		path = "grids"
	case "heroes", "hero", KindTitle:
		path = "heroes"
	case "logos", KindLogo, "wheel":
		path = "logos"
	default:
		return nil, classify(KindConfig, "未知 SteamGridDB 媒体类型: "+assetType, nil)
	}
	full := fmt.Sprintf(
		"https://www.steamgriddb.com/api/v2/%s/game/%d?types=static&nsfw=false&humor=false&limit=50",
		path, gameID,
	)
	raw, err := c.get(ctx, full)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Success bool `json:"success"`
		Data    []struct {
			URL    string `json:"url"`
			Thumb  string `json:"thumb"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
			Style  string `json:"style"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, classify(KindParse, "解析 SteamGridDB "+path+" 失败", err)
	}
	if !payload.Success {
		return nil, classify(KindNotFound, "SteamGridDB "+path+" 未命中", nil)
	}
	out := make([]MediaImage, 0, len(payload.Data))
	for _, it := range payload.Data {
		if strings.TrimSpace(it.URL) == "" {
			continue
		}
		out = append(out, MediaImage{
			URL:    it.URL,
			Thumb:  it.Thumb,
			Width:  it.Width,
			Height: it.Height,
			Style:  it.Style,
			Source: "steamgriddb",
		})
	}
	SortPreferJPG(out)
	return out, nil
}

// ResolveGameID finds the first autocomplete match for a name.
func (c *SGDBClient) ResolveGameID(ctx context.Context, name string) (int, string, error) {
	items, err := c.SearchGames(ctx, name, 1)
	if err != nil {
		return 0, "", err
	}
	if len(items) == 0 {
		return 0, "", classify(KindNotFound, "SteamGridDB 未命中", nil)
	}
	return items[0].GameID, items[0].GameName, nil
}

func (c *SGDBClient) get(ctx context.Context, fullURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, classify(KindNetwork, "构建 SteamGridDB 请求失败", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "Retrove/1.0")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, classify(KindNetwork, "请求 SteamGridDB 失败", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, classify(KindNetwork, "读取 SteamGridDB 响应失败", err)
	}
	switch res.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, classify(KindAuth, "SteamGridDB 鉴权失败，请检查 API Key", nil)
	case 429:
		return nil, classify(KindQuota, "SteamGridDB 限额", nil)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, classify(KindNetwork, fmt.Sprintf("SteamGridDB HTTP %d", res.StatusCode), nil)
	}
	return raw, nil
}
