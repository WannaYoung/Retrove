package translate

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"retrove/internal/config"
)

const batchSize = 20

type ProgressFunc func(done, total int, msg string)

type Options struct {
	PlatformID string
}

type Failure struct {
	GameID   int64  `json:"game_id"`
	Filename string `json:"filename"`
	Kind     string `json:"kind"`
	Message  string `json:"message"`
}

type Report struct {
	PlatformID string    `json:"platform_id"`
	Platform   string    `json:"platform"`
	Total      int       `json:"total"`
	Attempted  int       `json:"attempted"`
	Updated    int       `json:"updated"`
	Skipped    int       `json:"skipped"`
	Failed     []Failure `json:"failed,omitempty"`
}

type candidate struct {
	ID         int64
	TitleZH    string
	TitleEN    string
	Filename   string
	SourceText string
}

type Service struct {
	db  *sql.DB
	cfg config.Config
	llm *Client
}

func New(db *sql.DB, cfg config.Config) *Service {
	return &Service{
		db:  db,
		cfg: cfg,
		llm: NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model),
	}
}

func (s *Service) Client() *Client {
	if s == nil {
		return nil
	}
	return s.llm
}

func (s *Service) Run(ctx context.Context, opts Options, onProgress ProgressFunc) (Report, error) {
	rep := Report{PlatformID: opts.PlatformID}
	if strings.TrimSpace(opts.PlatformID) == "" {
		return rep, fmt.Errorf("需要 platform_id")
	}
	if s.llm == nil || !s.llm.Configured() {
		return rep, fmt.Errorf("未配置 LLM（需要 base_url / api_key / model）")
	}

	var platformName string
	err := s.db.QueryRow(`SELECT name FROM platforms WHERE id=?`, opts.PlatformID).Scan(&platformName)
	if err == sql.ErrNoRows {
		return rep, fmt.Errorf("平台不存在")
	}
	if err != nil {
		return rep, err
	}
	rep.Platform = platformName

	items, total, err := s.loadCandidates(opts.PlatformID)
	if err != nil {
		return rep, err
	}
	rep.Total = total
	if len(items) == 0 {
		if onProgress != nil {
			onProgress(0, 0, "无可翻译项（需要中文名且尚无英文名）")
		}
		return rep, nil
	}

	hint := PlatformHint(opts.PlatformID, platformName)
	sys := systemPrompt(hint)
	pending := len(items)
	done := 0
	if onProgress != nil {
		onProgress(0, pending, fmt.Sprintf("译英文 %s：0/%d", platformName, pending))
	}

	for i := 0; i < len(items); i += batchSize {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[i:end]
		translated, err := s.translateBatch(ctx, sys, batch)
		if err != nil {
			for _, c := range batch {
				rep.Attempted++
				rep.Failed = append(rep.Failed, Failure{
					GameID:   c.ID,
					Filename: c.Filename,
					Kind:     "llm",
					Message:  err.Error(),
				})
			}
			done = end
			if onProgress != nil {
				onProgress(done, pending, fmt.Sprintf("翻译失败：%s", truncate(err.Error(), 80)))
			}
			continue
		}
		for _, c := range batch {
			rep.Attempted++
			en := cleanTitle(translated[c.ID])
			if en == "" {
				rep.Failed = append(rep.Failed, Failure{
					GameID:   c.ID,
					Filename: c.Filename,
					Kind:     "empty",
					Message:  "模型未返回该条目",
				})
				continue
			}
			if looksMostlyChinese(en) {
				rep.Failed = append(rep.Failed, Failure{
					GameID:   c.ID,
					Filename: c.Filename,
					Kind:     "not_english",
					Message:  "模型未给出英文名: " + en,
				})
				continue
			}
			if strings.EqualFold(en, c.TitleEN) {
				rep.Skipped++
				continue
			}
			if _, err := s.db.Exec(`UPDATE games SET title_en=?, updated_at=datetime('now') WHERE id=?`, en, c.ID); err != nil {
				rep.Failed = append(rep.Failed, Failure{
					GameID:   c.ID,
					Filename: c.Filename,
					Kind:     "db",
					Message:  err.Error(),
				})
				continue
			}
			rep.Updated++
		}
		done = end
		if onProgress != nil {
			onProgress(done, pending, fmt.Sprintf("译英文 %s：%d/%d", platformName, done, pending))
		}
	}
	return rep, nil
}

func (s *Service) loadCandidates(platformID string) ([]candidate, int, error) {
	rows, err := s.db.Query(`
		SELECT g.id, g.title_zh, g.title_en, COALESCE(r.filename,'')
		FROM games g
		JOIN rom_files r ON r.game_id = g.id AND r.missing=0
		WHERE g.platform_id=?
		ORDER BY g.id`, platformID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []candidate
	seen := map[int64]bool{}
	total := 0
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.ID, &c.TitleZH, &c.TitleEN, &c.Filename); err != nil {
			return nil, 0, err
		}
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		total++
		src := strings.TrimSpace(c.TitleZH)
		if !looksMostlyChinese(src) {
			continue
		}
		if looksEnglish(c.TitleEN) {
			continue
		}
		c.SourceText = src
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (s *Service) translateBatch(ctx context.Context, sys string, batch []candidate) (map[int64]string, error) {
	type item struct {
		ID       int64  `json:"id"`
		TitleZH  string `json:"title_zh"`
		Filename string `json:"filename,omitempty"`
	}
	payload := make([]item, 0, len(batch))
	for _, c := range batch {
		payload = append(payload, item{ID: c.ID, TitleZH: c.SourceText, Filename: c.Filename})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	user := fmt.Sprintf(`Translate these Chinese game titles into English titles for scraper search. Input is a JSON array. Reply with a JSON object only: string id → English title.
Input:
%s`, string(raw))

	content, err := s.llm.Chat(ctx, sys, user)
	if err != nil {
		return nil, err
	}
	return parseTranslationMap(content)
}

func parseTranslationMap(content string) (map[int64]string, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```JSON")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var asMap map[string]string
	if err := json.Unmarshal([]byte(content), &asMap); err == nil {
		out := make(map[int64]string, len(asMap))
		for k, v := range asMap {
			var id int64
			if _, err := fmt.Sscan(k, &id); err != nil {
				continue
			}
			out[id] = strings.TrimSpace(v)
		}
		if len(out) > 0 {
			return out, nil
		}
	}

	var asArr []struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		EN    string `json:"en"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal([]byte(content), &asArr); err == nil && len(asArr) > 0 {
		out := make(map[int64]string, len(asArr))
		for _, row := range asArr {
			title := firstNonEmpty(row.Title, row.EN, row.Name)
			if row.ID > 0 && title != "" {
				out[row.ID] = strings.TrimSpace(title)
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}

	return nil, fmt.Errorf("无法解析模型返回的 JSON")
}

func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'“”‘’`)
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func looksMostlyChinese(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	han, other := 0, 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			han++
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			other++
		}
	}
	return han > 0 && han >= other
}

func looksEnglish(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || looksMostlyChinese(s) {
		return false
	}
	letters := 0
	for _, r := range s {
		if unicode.IsLetter(r) && r < unicode.MaxASCII {
			letters++
		}
	}
	return letters >= 2
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

var dumpNoise = regexp.MustCompile(`(?i)(\([^)]*\)|\[[^\]]*\]|\{[^}]*\}|!+)$`)

func cleanFilenameTitle(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, ".", " ")
	for {
		next := dumpNoise.ReplaceAllString(s, "")
		next = strings.TrimSpace(next)
		if next == s {
			break
		}
		s = next
	}
	return strings.Join(strings.Fields(s), " ")
}

func pickSource(titleEN, titleZH, filename string) string {
	if s := strings.TrimSpace(titleZH); looksMostlyChinese(s) {
		return s
	}
	if s := strings.TrimSpace(titleEN); s != "" {
		return s
	}
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	return strings.TrimSpace(cleanFilenameTitle(base))
}
