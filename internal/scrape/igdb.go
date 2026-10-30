package scrape

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type IGDBClient struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	minInterval  time.Duration

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
	lastCall    time.Time
}

func NewIGDBClient(clientID, clientSecret string) *IGDBClient {
	return &IGDBClient{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		minInterval:  280 * time.Millisecond, // stay under IGDB ~4 req/s
	}
}

func (c *IGDBClient) Configured() bool {
	return c != nil && c.clientID != "" && strings.TrimSpace(c.clientSecret) != ""
}

type IGDBGame struct {
	ID            int
	Name          string
	Summary       string
	Developer     string
	ReleaseYear   int
	CoverURL      string
	ArtworkURL    string
	ScreenshotURL string
	Covers        []MediaImage
	Artworks      []MediaImage
	Screenshots   []MediaImage
}

func (g IGDBGame) Bundle() MediaBundle {
	return MediaBundle{
		Box:        firstNonEmpty(g.CoverURL, PreferJPGURL(g.Covers)),
		Title:      firstNonEmpty(g.ArtworkURL, PreferJPGURL(g.Artworks)),
		Screenshot: firstNonEmpty(g.ScreenshotURL, PreferJPGURL(g.Screenshots)),
	}
}

const igdbGameFields = `name,summary,first_release_date,cover.image_id,cover.url,cover.width,cover.height,artworks.image_id,artworks.url,artworks.width,artworks.height,screenshots.image_id,screenshots.url,screenshots.width,screenshots.height,involved_companies.company.name,involved_companies.developer`

// Ping verifies Twitch client credentials by fetching an access token.
func (c *IGDBClient) Ping(ctx context.Context) error {
	if !c.Configured() {
		return classify(KindConfig, "未配置 IGDB", nil)
	}
	return c.ensureToken(ctx)
}

func (c *IGDBClient) GetGame(ctx context.Context, id int) (*IGDBGame, error) {
	if !c.Configured() {
		return nil, classify(KindConfig, "未配置 IGDB", nil)
	}
	if id <= 0 {
		return nil, classify(KindConfig, "无效 IGDB id", nil)
	}
	if err := c.ensureToken(ctx); err != nil {
		return nil, err
	}
	body := fmt.Sprintf(`fields %s; where id = %d;`, igdbGameFields, id)
	raw, err := c.post(ctx, "/games", body)
	if err != nil {
		return nil, err
	}
	games, err := parseIGDBGames(raw)
	if err != nil {
		return nil, err
	}
	if len(games) == 0 {
		return nil, classify(KindNotFound, "IGDB 未命中", nil)
	}
	return &games[0], nil
}

// MediaBundleForGame maps IGDB assets: cover→box, artwork→title, screenshot→screenshot (first each).
func (c *IGDBClient) MediaBundleForGame(ctx context.Context, gameID int) (MediaBundle, error) {
	g, err := c.GetGame(ctx, gameID)
	if err != nil {
		return MediaBundle{}, err
	}
	out := g.Bundle()
	// Nested fields are occasionally empty; fill gaps via asset endpoints.
	if out.Box == "" {
		if covers, err := c.ListAssets(ctx, gameID, KindBox); err == nil {
			out.Box = PreferJPGURL(covers)
		} else if AsKind(err) == KindQuota || AsKind(err) == KindAuth {
			return out, err
		}
	}
	if out.Title == "" {
		if arts, err := c.ListAssets(ctx, gameID, KindTitle); err == nil {
			out.Title = PreferJPGURL(arts)
		} else if AsKind(err) == KindQuota || AsKind(err) == KindAuth {
			return out, err
		}
	}
	if out.Screenshot == "" {
		if shots, err := c.ListAssets(ctx, gameID, KindScreenshot); err == nil {
			out.Screenshot = PreferJPGURL(shots)
		} else if AsKind(err) == KindQuota || AsKind(err) == KindAuth {
			return out, err
		}
	}
	if out.Box == "" && out.Title == "" && out.Screenshot == "" {
		return out, classify(KindNotFound, "IGDB 无可用媒体", nil)
	}
	return out, nil
}

// ListAssets returns covers / artworks / screenshots for an IGDB game.
func (c *IGDBClient) ListAssets(ctx context.Context, gameID int, assetType string) ([]MediaImage, error) {
	if !c.Configured() {
		return nil, classify(KindConfig, "未配置 IGDB", nil)
	}
	if gameID <= 0 {
		return nil, classify(KindConfig, "无效 IGDB id", nil)
	}
	if err := c.ensureToken(ctx); err != nil {
		return nil, err
	}
	assetType = strings.ToLower(strings.TrimSpace(assetType))
	var path string
	switch assetType {
	case "covers", "cover", KindBox:
		path = "/covers"
	case "artworks", "artwork", KindTitle:
		path = "/artworks"
	case "screenshots", KindScreenshot:
		path = "/screenshots"
	default:
		return nil, classify(KindConfig, "未知 IGDB 媒体类型: "+assetType, nil)
	}
	body := fmt.Sprintf(`fields image_id,url,width,height; where game = %d; limit 50;`, gameID)
	raw, err := c.post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	rows, err := parseIGDBImages(raw)
	if err != nil {
		return nil, classify(KindParse, "解析 IGDB "+path+" 失败", err)
	}
	SortPreferJPG(rows)
	return rows, nil
}

// ResolveGameID finds the first search match for a name.
func (c *IGDBClient) ResolveGameID(ctx context.Context, name string) (int, string, error) {
	games, err := c.SearchGames(ctx, name, 1)
	if err != nil {
		return 0, "", err
	}
	if len(games) == 0 {
		return 0, "", classify(KindNotFound, "IGDB 未命中", nil)
	}
	return games[0].ID, games[0].Name, nil
}

type igdbImageRow struct {
	ImageID string `json:"image_id"`
	URL     string `json:"url"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

func parseIGDBImages(raw []byte) ([]MediaImage, error) {
	var rows []igdbImageRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	out := make([]MediaImage, 0, len(rows))
	for _, it := range rows {
		full := igdbImageURL(it.ImageID, it.URL, "t_original")
		if full == "" {
			continue
		}
		out = append(out, MediaImage{
			URL:    full,
			Thumb:  igdbImageURL(it.ImageID, it.URL, "t_thumb"),
			Width:  it.Width,
			Height: it.Height,
			Source: "igdb",
		})
	}
	return out, nil
}

func imagesFromRows(rows []igdbImageRow) []MediaImage {
	out := make([]MediaImage, 0, len(rows))
	for _, it := range rows {
		full := igdbImageURL(it.ImageID, it.URL, "t_original")
		if full == "" {
			continue
		}
		out = append(out, MediaImage{
			URL:    full,
			Thumb:  igdbImageURL(it.ImageID, it.URL, "t_thumb"),
			Width:  it.Width,
			Height: it.Height,
			Source: "igdb",
		})
	}
	SortPreferJPG(out)
	return out
}

func parseIGDBGames(raw []byte) ([]IGDBGame, error) {
	var rows []struct {
		ID               int           `json:"id"`
		Name             string        `json:"name"`
		Summary          string        `json:"summary"`
		FirstReleaseDate int64         `json:"first_release_date"`
		Cover            *igdbImageRow `json:"cover"`
		Artworks         []igdbImageRow `json:"artworks"`
		Screenshots      []igdbImageRow `json:"screenshots"`
		InvolvedCompanies []struct {
			Developer bool `json:"developer"`
			Company   *struct {
				Name string `json:"name"`
			} `json:"company"`
		} `json:"involved_companies"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, classify(KindParse, "解析 IGDB 响应失败", err)
	}
	out := make([]IGDBGame, 0, len(rows))
	for _, best := range rows {
		g := IGDBGame{
			ID:      best.ID,
			Name:    best.Name,
			Summary: best.Summary,
		}
		if best.FirstReleaseDate > 0 {
			g.ReleaseYear = time.Unix(best.FirstReleaseDate, 0).UTC().Year()
		}
		for _, ic := range best.InvolvedCompanies {
			if ic.Developer && ic.Company != nil && ic.Company.Name != "" {
				g.Developer = ic.Company.Name
				break
			}
		}
		if g.Developer == "" {
			for _, ic := range best.InvolvedCompanies {
				if ic.Company != nil && ic.Company.Name != "" {
					g.Developer = ic.Company.Name
					break
				}
			}
		}
		if best.Cover != nil {
			g.Covers = imagesFromRows([]igdbImageRow{*best.Cover})
			if len(g.Covers) > 0 {
				g.CoverURL = g.Covers[0].URL
			} else {
				g.CoverURL = igdbCoverURL(best.Cover.URL)
			}
		}
		g.Artworks = imagesFromRows(best.Artworks)
		if len(g.Artworks) > 0 {
			g.ArtworkURL = g.Artworks[0].URL
		}
		g.Screenshots = imagesFromRows(best.Screenshots)
		if len(g.Screenshots) > 0 {
			g.ScreenshotURL = g.Screenshots[0].URL
		}
		out = append(out, g)
	}
	return out, nil
}

func (c *IGDBClient) SearchGame(ctx context.Context, name string) (*IGDBGame, error) {
	return c.SearchGamePrefer(ctx, name, false)
}

// SearchGamePrefer searches up to 5 hits. When preferScreenshot is set, picks the first hit that has screenshots.
func (c *IGDBClient) SearchGamePrefer(ctx context.Context, name string, preferScreenshot bool) (*IGDBGame, error) {
	games, err := c.SearchGames(ctx, name, 5)
	if err != nil {
		return nil, err
	}
	if len(games) == 0 {
		return nil, classify(KindNotFound, "IGDB 未命中", nil)
	}
	if preferScreenshot {
		for i := range games {
			if games[i].ScreenshotURL != "" || len(games[i].Screenshots) > 0 {
				return &games[i], nil
			}
		}
	}
	return &games[0], nil
}

func (c *IGDBClient) SearchGames(ctx context.Context, name string, limit int) ([]IGDBGame, error) {
	if !c.Configured() {
		return nil, classify(KindConfig, "未配置 IGDB", nil)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, classify(KindNotFound, "无可用英文名搜索 IGDB", nil)
	}
	if limit <= 0 {
		limit = 10
	}
	if err := c.ensureToken(ctx); err != nil {
		return nil, err
	}

	escaped := strings.ReplaceAll(name, `"`, "")
	body := fmt.Sprintf(`search "%s"; fields %s; limit %d;`, escaped, igdbGameFields, limit)
	raw, err := c.post(ctx, "/games", body)
	if err != nil {
		return nil, err
	}
	out, err := parseIGDBGames(raw)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, classify(KindNotFound, "IGDB 未命中", nil)
	}
	return out, nil
}

func (c *IGDBClient) ensureToken(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry.Add(-60*time.Second)) {
		return nil
	}
	u := "https://id.twitch.tv/oauth2/token?" + url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"grant_type":    {"client_credentials"},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return classify(KindNetwork, "构建 IGDB token 请求失败", err)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return classify(KindNetwork, "请求 IGDB token 失败", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return classify(KindAuth, "IGDB 鉴权失败，请检查 Client ID/Secret", nil)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return classify(KindNetwork, fmt.Sprintf("IGDB token HTTP %d", res.StatusCode), nil)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil || tok.AccessToken == "" {
		return classify(KindParse, "解析 IGDB token 失败", err)
	}
	c.accessToken = tok.AccessToken
	exp := tok.ExpiresIn
	if exp <= 0 {
		exp = 3600
	}
	c.tokenExpiry = time.Now().Add(time.Duration(exp) * time.Second)
	return nil
}

func (c *IGDBClient) wait(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	wait := c.minInterval - time.Since(c.lastCall)
	if wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return classify(KindNetwork, "cancelled", ctx.Err())
		case <-t.C:
		}
	}
	c.lastCall = time.Now()
	return nil
}

func (c *IGDBClient) post(ctx context.Context, path, body string) ([]byte, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.igdb.com/v4"+path, strings.NewReader(body))
	if err != nil {
		return nil, classify(KindNetwork, "构建 IGDB 请求失败", err)
	}
	c.mu.Lock()
	token := c.accessToken
	c.mu.Unlock()
	req.Header.Set("Client-ID", c.clientID)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "text/plain")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, classify(KindNetwork, "请求 IGDB 失败", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, classify(KindNetwork, "读取 IGDB 响应失败", err)
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, classify(KindAuth, "IGDB 鉴权失败", nil)
	}
	if res.StatusCode == 429 {
		return nil, classify(KindQuota, "IGDB 限额", nil)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, classify(KindNetwork, fmt.Sprintf("IGDB HTTP %d: %s", res.StatusCode, truncate(string(raw), 160)), nil)
	}
	return raw, nil
}

func igdbCoverURL(u string) string {
	return igdbImageURL("", u, "t_cover_big")
}

func igdbImageURL(imageID, fallbackURL, size string) string {
	imageID = strings.TrimSpace(imageID)
	size = strings.TrimSpace(size)
	if size == "" {
		size = "t_original"
	}
	if imageID != "" {
		return "https://images.igdb.com/igdb/image/upload/" + size + "/" + imageID + ".jpg"
	}
	u := strings.TrimSpace(fallbackURL)
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}
	if size != "t_thumb" {
		u = strings.Replace(u, "/t_thumb/", "/"+size+"/", 1)
	}
	return u
}

func formatIGDBID(id int) string {
	return "igdb:" + strconv.Itoa(id)
}

func parseProviderID(scrapeID, provider string) (int, bool) {
	scrapeID = strings.TrimSpace(scrapeID)
	prefix := ""
	switch strings.ToLower(provider) {
	case "igdb":
		prefix = "igdb:"
	case "steamgriddb", "sgdb":
		prefix = "steamgriddb:"
		if strings.HasPrefix(strings.ToLower(scrapeID), "sgdb:") {
			prefix = "sgdb:"
		}
	case "screenscraper", "ss":
		prefix = "ss:"
	default:
		return 0, false
	}
	if !strings.HasPrefix(strings.ToLower(scrapeID), strings.ToLower(prefix)) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(scrapeID[len(prefix):]))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
