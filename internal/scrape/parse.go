package scrape

import (
	"encoding/json"
	"strconv"
	"strings"
)

// GameInfo is a normalized ScreenScraper jeu payload.
type GameInfo struct {
	ID          string
	TitleEN     string
	TitleJA     string
	TitleZH     string
	Developer   string
	Publisher   string
	Description string
	ReleaseYear int
	Medias      []MediaRef
}

type MediaRef struct {
	Type   string
	URL    string
	Region string
	Format string
}

func parseJeuRecherche(raw []byte) ([]*GameInfo, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, classify(KindParse, "响应不是 JSON", err)
	}
	respRaw, ok := root["response"]
	if !ok {
		return nil, classify(KindNotFound, "未命中", nil)
	}
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(respRaw, &resp); err != nil {
		return nil, classify(KindParse, "解析 response 失败", err)
	}
	jeuxRaw, ok := resp["jeux"]
	if !ok || string(jeuxRaw) == "null" || string(jeuxRaw) == `""` {
		return []*GameInfo{}, nil
	}
	var list []json.RawMessage
	if err := json.Unmarshal(jeuxRaw, &list); err != nil {
		// single object
		info, err := parseJeuObject(jeuxRaw)
		if err != nil {
			return []*GameInfo{}, nil
		}
		return []*GameInfo{info}, nil
	}
	out := make([]*GameInfo, 0, len(list))
	for _, item := range list {
		info, err := parseJeuObject(item)
		if err != nil || info == nil || info.ID == "" {
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

func parseJeuObject(jeuRaw json.RawMessage) (*GameInfo, error) {
	if string(jeuRaw) == "null" || string(jeuRaw) == `""` || string(jeuRaw) == "{}" || string(jeuRaw) == "[]" {
		return nil, classify(KindNotFound, "未命中", nil)
	}
	var jeu map[string]json.RawMessage
	if err := json.Unmarshal(jeuRaw, &jeu); err != nil {
		return nil, classify(KindParse, "解析 jeu 失败", err)
	}
	info := &GameInfo{
		ID:        textField(jeu["id"]),
		Developer: nestedText(jeu["developpeur"]),
		Publisher: nestedText(jeu["editeur"]),
	}
	if info.ID == "" {
		return nil, classify(KindNotFound, "未命中", nil)
	}
	info.TitleEN, info.TitleJA, info.TitleZH = pickTitles(jeu["noms"], jeu["nom"])
	info.Description = pickSynopsis(jeu["synopsis"])
	info.ReleaseYear = pickYear(jeu["dates"])
	info.Medias = parseMedias(jeu["medias"])
	return info, nil
}

func parseJeuInfos(raw []byte) (*GameInfo, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, classify(KindParse, "响应不是 JSON", err)
	}
	respRaw, ok := root["response"]
	if !ok {
		return nil, classify(KindNotFound, "未命中", nil)
	}
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(respRaw, &resp); err != nil {
		return nil, classify(KindParse, "解析 response 失败", err)
	}
	jeuRaw, ok := resp["jeu"]
	if !ok || string(jeuRaw) == "null" || string(jeuRaw) == `""` || string(jeuRaw) == "[]" {
		return nil, classify(KindNotFound, "未命中", nil)
	}
	return parseJeuObject(jeuRaw)
}

func textField(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return strconv.FormatInt(int64(f), 10)
	}
	return nestedText(raw)
}

func nestedText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		if t, ok := obj["text"]; ok {
			return textField(t)
		}
		if t, ok := obj["nom"]; ok {
			return textField(t)
		}
	}
	return ""
}

func pickTitles(nomsRaw, nomRaw json.RawMessage) (en, ja, zh string) {
	type named struct {
		Region string `json:"region"`
		Langue string `json:"langue"`
		Text   string `json:"text"`
	}
	var list []named
	if err := json.Unmarshal(nomsRaw, &list); err == nil && len(list) > 0 {
		byRegion := map[string]string{}
		for _, n := range list {
			key := strings.ToLower(n.Region)
			if key == "" {
				key = strings.ToLower(n.Langue)
			}
			byRegion[key] = strings.TrimSpace(n.Text)
		}
		en = firstNonEmpty(byRegion["us"], byRegion["wor"], byRegion["eu"], byRegion["ss"], byRegion["en"])
		ja = firstNonEmpty(byRegion["jp"], byRegion["ja"], byRegion["asi"])
		zh = firstNonEmpty(byRegion["cn"], byRegion["tw"], byRegion["zh"], byRegion["zhcn"], byRegion["zhtw"])
		if en == "" {
			for _, n := range list {
				if strings.TrimSpace(n.Text) != "" {
					en = strings.TrimSpace(n.Text)
					break
				}
			}
		}
		return en, ja, zh
	}

	// Object form: nom_us / nom_eu / …
	var obj map[string]string
	if err := json.Unmarshal(nomsRaw, &obj); err == nil && len(obj) > 0 {
		en = firstNonEmpty(obj["nom_us"], obj["nom_wor"], obj["nom_eu"], obj["nom_ss"], obj["nom_en"])
		ja = firstNonEmpty(obj["nom_jp"], obj["nom_ja"])
		zh = firstNonEmpty(obj["nom_cn"], obj["nom_tw"], obj["nom_zh"])
		if en == "" {
			for _, v := range obj {
				if strings.TrimSpace(v) != "" {
					en = strings.TrimSpace(v)
					break
				}
			}
		}
		return en, ja, zh
	}

	en = nestedText(nomRaw)
	return en, ja, zh
}

func pickSynopsis(raw json.RawMessage) string {
	type syn struct {
		Langue string `json:"langue"`
		Text   string `json:"text"`
	}
	var list []syn
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		byLang := map[string]string{}
		for _, s := range list {
			byLang[strings.ToLower(s.Langue)] = strings.TrimSpace(s.Text)
		}
		return firstNonEmpty(byLang["zh"], byLang["cn"], byLang["en"], byLang["us"], byLang["fr"])
	}
	var obj map[string]string
	if err := json.Unmarshal(raw, &obj); err == nil {
		return firstNonEmpty(obj["synopsis_zh"], obj["synopsis_cn"], obj["synopsis_en"], obj["synopsis_fr"])
	}
	return nestedText(raw)
}

func pickYear(raw json.RawMessage) int {
	type dated struct {
		Region string `json:"region"`
		Text   string `json:"text"`
	}
	var list []dated
	if err := json.Unmarshal(raw, &list); err == nil {
		for _, d := range list {
			if y := yearFrom(d.Text); y > 0 {
				return y
			}
		}
	}
	var obj map[string]string
	if err := json.Unmarshal(raw, &obj); err == nil {
		for _, v := range obj {
			if y := yearFrom(v); y > 0 {
				return y
			}
		}
	}
	return yearFrom(nestedText(raw))
}

func yearFrom(s string) int {
	s = strings.TrimSpace(s)
	if len(s) >= 4 {
		y, err := strconv.Atoi(s[:4])
		if err == nil && y >= 1970 && y <= 2100 {
			return y
		}
	}
	return 0
}

func parseMedias(raw json.RawMessage) []MediaRef {
	type media struct {
		Type   string `json:"type"`
		URL    string `json:"url"`
		Region string `json:"region"`
		Format string `json:"format"`
	}
	var list []media
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	out := make([]MediaRef, 0, len(list))
	for _, m := range list {
		url := strings.TrimSpace(m.URL)
		if url == "" || m.Type == "" {
			continue
		}
		out = append(out, MediaRef{
			Type:   m.Type,
			URL:    url,
			Region: m.Region,
			Format: m.Format,
		})
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func mediaTypePrefs(kind string) []string {
	switch kind {
	case KindBox:
		return []string{"box-2D", "box-2D-reconstructed", "box-3D", "box-texture"}
	case KindScreenshot:
		// gameplay screenshots only; sstitle is title-screen art and belongs to KindTitle
		return []string{"ss", "screenshot"}
	case KindTitle:
		return []string{"sstitle", "title", "screenmarquee"}
	case KindLogo, "wheel":
		return []string{"wheel-hd", "wheel", "wheel-carbon", "wheel-steel"}
	default:
		return []string{kind}
	}
}

func regionScore(r string) int {
	switch strings.ToLower(r) {
	case "wor", "us", "eu", "ss":
		return 3
	case "jp", "asi":
		return 2
	case "":
		return 1
	default:
		return 0
	}
}

// formatScore prefers jpg over other image formats. It is a tie-break within the same media type.
func formatScore(format, mediaURL string) int {
	f := strings.ToLower(strings.TrimSpace(format))
	u := strings.ToLower(mediaURL)
	switch {
	case f == "jpg" || f == "jpeg" || strings.Contains(u, ".jpg") || strings.Contains(u, ".jpeg"):
		return 2
	case f == "webp" || strings.Contains(u, ".webp"):
		return 1
	default:
		return 0
	}
}

func mediaScore(typeRank, typeCount int, format, region, mediaURL string) int {
	return (typeCount-typeRank)*100 + formatScore(format, mediaURL)*10 + regionScore(region)
}

// PreferMedia picks best media URL for a logical kind (box/screenshot/title/logo).
// Same media type prefers jpg.
func PreferMedia(medias []MediaRef, kind string) (MediaRef, bool) {
	if kind == "wheel" {
		kind = KindLogo
	}
	typePref := mediaTypePrefs(kind)
	var best MediaRef
	bestScore := -1
	for i, want := range typePref {
		for _, m := range medias {
			if !strings.EqualFold(m.Type, want) {
				continue
			}
			score := mediaScore(i, len(typePref), m.Format, m.Region, m.URL)
			if score > bestScore {
				bestScore = score
				best = m
			}
		}
	}
	return best, bestScore >= 0
}

// ListMediasForKind returns all ScreenScraper images for a logical kind, best first.
func ListMediasForKind(medias []MediaRef, kind string) []MediaImage {
	typePref := mediaTypePrefs(kind)
	type scored struct {
		img   MediaImage
		score int
	}
	seen := map[string]bool{}
	var list []scored
	for i, want := range typePref {
		for _, m := range medias {
			if !strings.EqualFold(m.Type, want) || m.URL == "" || seen[m.URL] {
				continue
			}
			seen[m.URL] = true
			list = append(list, scored{
				img: MediaImage{
					URL:    m.URL,
					Style:  m.Type,
					Source: "screenscraper",
				},
				score: mediaScore(i, len(typePref), m.Format, m.Region, m.URL),
			})
		}
	}
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].score > list[i].score {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	out := make([]MediaImage, 0, len(list))
	for _, it := range list {
		out = append(out, it.img)
	}
	return out
}

// BundleFromSSMedias maps ScreenScraper medias to a first-image MediaBundle.
func BundleFromSSMedias(medias []MediaRef) MediaBundle {
	var out MediaBundle
	for _, kind := range []string{KindBox, KindScreenshot, KindTitle, KindLogo} {
		if ref, ok := PreferMedia(medias, kind); ok {
			out.Set(kind, ref.URL)
		}
	}
	return out
}
