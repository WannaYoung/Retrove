package scrape

import (
	"sort"
	"strings"
)

// Logical media kinds stored in media_assets.
const (
	KindBox        = "box"        // 主封面
	KindScreenshot = "screenshot" // 截图
	KindTitle      = "title"      // 标题图
	KindLogo       = "logo"       // Logo（ScreenScraper 类型仍为 wheel*）
)

// ProviderSupportsKind reports whether a scrape source can supply a media kind.
func ProviderSupportsKind(provider, kind string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	k := strings.ToLower(strings.TrimSpace(kind))
	if k == "wheel" {
		k = KindLogo
	}
	switch p {
	case "screenscraper", "ss":
		return k == KindBox || k == KindScreenshot || k == KindTitle || k == KindLogo
	case "steamgriddb", "sgdb":
		// grids→box, heroes→title, logos→logo; no native screenshots
		return k == KindBox || k == KindTitle || k == KindLogo
	case "igdb":
		// cover→box, artwork→title, screenshot→screenshot; no logo
		return k == KindBox || k == KindTitle || k == KindScreenshot
	default:
		return false
	}
}

// MediaImage is one remote image candidate for browse/pick UI.
type MediaImage struct {
	URL    string `json:"url"`
	Thumb  string `json:"thumb,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Style  string `json:"style,omitempty"`
	Source string `json:"source,omitempty"`
}

// MediaBundle holds first-image URLs for scrape (batch / manual).
type MediaBundle struct {
	Box        string
	Screenshot string
	Title      string
	Logo       string
}

func (m MediaBundle) URLFor(kind string) string {
	switch kind {
	case KindBox:
		return m.Box
	case KindScreenshot:
		return m.Screenshot
	case KindTitle:
		return m.Title
	case KindLogo, "wheel":
		return m.Logo
	default:
		return ""
	}
}

func (m *MediaBundle) Set(kind, url string) {
	url = strings.TrimSpace(url)
	if url == "" {
		return
	}
	switch kind {
	case KindBox:
		m.Box = url
	case KindScreenshot:
		m.Screenshot = url
	case KindTitle:
		m.Title = url
	case KindLogo, "wheel":
		m.Logo = url
	}
}

// SortPreferJPG moves jpg/jpeg candidates ahead of other formats.
func SortPreferJPG(items []MediaImage) {
	sort.SliceStable(items, func(i, j int) bool {
		return formatScore("", items[i].URL) > formatScore("", items[j].URL)
	})
}

// PreferJPGURL returns the first jpg/jpeg URL, otherwise the first item.
func PreferJPGURL(items []MediaImage) string {
	for _, it := range items {
		if formatScore("", it.URL) >= 2 && strings.TrimSpace(it.URL) != "" {
			return it.URL
		}
	}
	for _, it := range items {
		if strings.TrimSpace(it.URL) != "" {
			return it.URL
		}
	}
	return ""
}

// MergeFill copies non-empty URLs from src into m only where m is empty.
func (m *MediaBundle) MergeFill(src MediaBundle) {
	if m.Box == "" {
		m.Box = src.Box
	}
	if m.Screenshot == "" {
		m.Screenshot = src.Screenshot
	}
	if m.Title == "" {
		m.Title = src.Title
	}
	if m.Logo == "" {
		m.Logo = src.Logo
	}
}
