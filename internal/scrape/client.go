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

const defaultBaseURL = "https://api.screenscraper.fr/api2"

// Credentials for ScreenScraper API.
type Credentials struct {
	DevID       string
	DevPassword string
	UserSS      string
	UserPass    string
	Softname    string
}

func (c Credentials) Configured() bool {
	return strings.TrimSpace(c.DevID) != "" && strings.TrimSpace(c.DevPassword) != ""
}

// Client talks to ScreenScraper with simple rate limiting.
type Client struct {
	creds       Credentials
	baseURL     string
	httpClient  *http.Client
	minInterval time.Duration

	mu       sync.Mutex
	lastCall time.Time
}

func NewClient(creds Credentials) *Client {
	soft := creds.Softname
	if soft == "" {
		soft = "Retrove"
	}
	creds.Softname = soft
	return &Client{
		creds:       creds,
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: 45 * time.Second},
		minInterval: 1500 * time.Millisecond,
	}
}

// LookupQuery is a jeuInfos request.
type LookupQuery struct {
	SystemID int
	CRC32    string
	MD5      string
	SHA1     string
	Filename string
	Size     int64
	RomType  string
}

func (c *Client) wait(ctx context.Context) error {
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

func (c *Client) commonQuery() url.Values {
	q := url.Values{}
	q.Set("devid", c.creds.DevID)
	q.Set("devpassword", c.creds.DevPassword)
	q.Set("softname", c.creds.Softname)
	q.Set("output", "json")
	if c.creds.UserSS != "" {
		q.Set("ssid", c.creds.UserSS)
		q.Set("sspassword", c.creds.UserPass)
	}
	return q
}

// Ping verifies developer credentials via systemesListe.
func (c *Client) Ping(ctx context.Context) error {
	if !c.creds.Configured() {
		return classify(KindConfig, "未配置 ScreenScraper 开发者凭据", nil)
	}
	if err := c.wait(ctx); err != nil {
		return err
	}
	_, err := c.get(ctx, "/systemesListe.php", c.commonQuery())
	return err
}

// Search finds games by name via jeuRecherche.php.
func (c *Client) Search(ctx context.Context, name string, systemID int) ([]*GameInfo, error) {
	if !c.creds.Configured() {
		return nil, classify(KindConfig, "未配置 ScreenScraper 开发者凭据", nil)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, classify(KindNotFound, "无可用搜索词", nil)
	}
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	q := c.commonQuery()
	q.Set("recherche", name)
	if systemID > 0 {
		q.Set("systemeid", strconv.Itoa(systemID))
	}
	raw, err := c.get(ctx, "/jeuRecherche.php", q)
	if err != nil {
		return nil, err
	}
	return parseJeuRecherche(raw)
}

// LookupByID fetches full jeuInfos for a ScreenScraper game id.
func (c *Client) LookupByID(ctx context.Context, gameID string) (*GameInfo, []byte, error) {
	if !c.creds.Configured() {
		return nil, nil, classify(KindConfig, "未配置 ScreenScraper 开发者凭据", nil)
	}
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return nil, nil, classify(KindConfig, "需要 game id", nil)
	}
	if err := c.wait(ctx); err != nil {
		return nil, nil, err
	}
	q := c.commonQuery()
	q.Set("gameid", gameID)
	raw, err := c.get(ctx, "/jeuInfos.php", q)
	if err != nil {
		return nil, nil, err
	}
	info, err := parseJeuInfos(raw)
	if err != nil {
		return nil, raw, err
	}
	return info, raw, nil
}

func (c *Client) Lookup(ctx context.Context, query LookupQuery) (*GameInfo, []byte, error) {
	if !c.creds.Configured() {
		return nil, nil, classify(KindConfig, "未配置 ScreenScraper 开发者凭据", nil)
	}
	if query.SystemID <= 0 {
		return nil, nil, classify(KindConfig, "平台未配置 screenscraper_id", nil)
	}
	if err := c.wait(ctx); err != nil {
		return nil, nil, err
	}

	q := c.commonQuery()
	q.Set("systemeid", strconv.Itoa(query.SystemID))
	romType := query.RomType
	if romType == "" {
		romType = "rom"
	}
	q.Set("romtype", romType)
	if query.CRC32 != "" {
		q.Set("crc", strings.ToUpper(query.CRC32))
	}
	if query.MD5 != "" {
		q.Set("md5", strings.ToUpper(query.MD5))
	}
	if query.SHA1 != "" {
		q.Set("sha1", strings.ToUpper(query.SHA1))
	}
	if query.Filename != "" {
		q.Set("romnom", query.Filename)
	}
	if query.Size > 0 {
		q.Set("romtaille", strconv.FormatInt(query.Size, 10))
	}

	raw, err := c.get(ctx, "/jeuInfos.php", q)
	if err != nil {
		return nil, nil, err
	}
	info, err := parseJeuInfos(raw)
	if err != nil {
		return nil, raw, err
	}
	return info, raw, nil
}

func (c *Client) Download(ctx context.Context, mediaURL string) ([]byte, string, error) {
	if mediaURL == "" {
		return nil, "", classify(KindMedia, "空媒体 URL", nil)
	}
	if err := c.wait(ctx); err != nil {
		return nil, "", err
	}
	u, err := url.Parse(mediaURL)
	if err != nil {
		return nil, "", classify(KindMedia, "无效媒体 URL", err)
	}
	// Media CDN links often already include auth; merge missing SS params when needed.
	q := u.Query()
	base := c.commonQuery()
	for k, vs := range base {
		if q.Get(k) == "" && len(vs) > 0 {
			q.Set(k, vs[0])
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", classify(KindNetwork, "构建下载请求失败", err)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", classify(KindNetwork, "下载媒体失败", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return nil, "", classify(KindNetwork, "读取媒体失败", err)
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, "", classify(KindAuth, fmt.Sprintf("媒体鉴权失败 HTTP %d", res.StatusCode), nil)
	}
	if res.StatusCode == 429 || res.StatusCode == 430 {
		return nil, "", classify(KindQuota, "媒体下载触发限额", nil)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, "", classify(KindMedia, fmt.Sprintf("媒体下载 HTTP %d", res.StatusCode), nil)
	}
	ct := res.Header.Get("Content-Type")
	return body, ct, nil
}

func (c *Client) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	u := c.baseURL + path + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, classify(KindNetwork, "构建请求失败", err)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, classify(KindNetwork, "请求 ScreenScraper 失败", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, classify(KindNetwork, "读取响应失败", err)
	}

	switch res.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, classify(KindAuth, fmt.Sprintf("鉴权失败 HTTP %d", res.StatusCode), nil)
	case 429, 430:
		return nil, classify(KindQuota, "API 限额已用尽", nil)
	case http.StatusNotFound:
		return nil, classify(KindNotFound, "未命中", nil)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if looksLikeQuota(msg) {
			return nil, classify(KindQuota, truncate(msg, 200), nil)
		}
		if looksLikeAuth(msg) {
			return nil, classify(KindAuth, truncate(msg, 200), nil)
		}
		if looksLikeNotFound(msg) {
			return nil, classify(KindNotFound, "未命中", nil)
		}
		return nil, classify(KindNetwork, fmt.Sprintf("HTTP %d: %s", res.StatusCode, truncate(msg, 200)), nil)
	}

	// Even 200 responses may encode API errors in JSON header.
	if err := detectAPIError(body); err != nil {
		return nil, err
	}
	return body, nil
}

func detectAPIError(body []byte) error {
	var envelope struct {
		Header struct {
			Erreur string `json:"erreur"`
			Error  string `json:"error"`
			Code   any    `json:"code"`
		} `json:"header"`
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		// Non-JSON body (plain text errors)
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			return nil
		}
		if looksLikeQuota(msg) {
			return classify(KindQuota, truncate(msg, 200), nil)
		}
		if looksLikeAuth(msg) {
			return classify(KindAuth, truncate(msg, 200), nil)
		}
		if looksLikeNotFound(msg) {
			return classify(KindNotFound, "未命中", nil)
		}
		return nil
	}
	msg := strings.TrimSpace(envelope.Header.Erreur)
	if msg == "" {
		msg = strings.TrimSpace(envelope.Header.Error)
	}
	if msg == "" {
		return nil
	}
	if looksLikeQuota(msg) || codeIs(envelope.Header.Code, 430) {
		return classify(KindQuota, truncate(msg, 200), nil)
	}
	if looksLikeAuth(msg) {
		return classify(KindAuth, truncate(msg, 200), nil)
	}
	if looksLikeNotFound(msg) || codeIs(envelope.Header.Code, 404) {
		return classify(KindNotFound, "未命中", nil)
	}
	return classify(KindParse, truncate(msg, 200), nil)
}

func looksLikeQuota(msg string) bool {
	l := strings.ToLower(msg)
	return strings.Contains(l, "quota") || strings.Contains(l, "430") ||
		strings.Contains(msg, "dépassé") || strings.Contains(msg, "depassé") ||
		strings.Contains(msg, "限额")
}

func looksLikeAuth(msg string) bool {
	l := strings.ToLower(msg)
	return strings.Contains(l, "password") || strings.Contains(l, "login") ||
		strings.Contains(l, "identifiant") || strings.Contains(l, "dev") && strings.Contains(l, "incorrect") ||
		strings.Contains(msg, "鉴权") || strings.Contains(msg, "未授权")
}

func looksLikeNotFound(msg string) bool {
	l := strings.ToLower(msg)
	return strings.Contains(l, "introuvable") || strings.Contains(l, "not found") ||
		strings.Contains(l, "aucune") || strings.Contains(l, "unknown game") ||
		strings.Contains(msg, "未找到")
}

func codeIs(v any, want int) bool {
	switch t := v.(type) {
	case float64:
		return int(t) == want
	case string:
		n, _ := strconv.Atoi(t)
		return n == want
	case json.Number:
		n, _ := t.Int64()
		return int(n) == want
	default:
		return false
	}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
