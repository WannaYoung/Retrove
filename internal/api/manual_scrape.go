package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"retrove/internal/library"
	"retrove/internal/scrape"
)

func (s *Server) searchGameScrape(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "validation", "无效 id")
		return
	}
	var body struct {
		Provider string `json:"provider"`
		Query    string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	g, err := s.lib.GetGame(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "游戏不存在")
		return
	}
	query := strings.TrimSpace(body.Query)
	if query == "" {
		query = g.TitleEN
	}
	if query == "" {
		writeError(w, http.StatusBadRequest, "validation", "需要英文名")
		return
	}

	s.mu.RLock()
	svc := scrape.New(s.db, s.cfg)
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	items, err := svc.SearchProvider(ctx, body.Provider, query, g.PlatformID)
	if err != nil {
		msg := err.Error()
		if e, ok := err.(*scrape.Error); ok {
			msg = e.Message
		}
		writeError(w, http.StatusBadGateway, "scrape", msg)
		return
	}
	if items == nil {
		items = []scrape.Candidate{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "provider": body.Provider, "query": query})
}

func (s *Server) previewGameScrape(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "validation", "无效 id")
		return
	}
	var body struct {
		Provider string `json:"provider"`
		ID       string `json:"candidate_id"`
		Query    string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	g, err := s.lib.GetGame(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "游戏不存在")
		return
	}

	s.mu.RLock()
	svc := scrape.New(s.db, s.cfg)
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	cand, err := svc.PreviewCandidate(ctx, body.Provider, body.ID, firstNonEmptyStr(body.Query, g.TitleEN), g.PlatformID)
	if err != nil {
		msg := err.Error()
		if e, ok := err.(*scrape.Error); ok {
			msg = e.Message
		}
		writeError(w, http.StatusBadGateway, "scrape", msg)
		return
	}
	choices := scrape.DefaultChoices(g, cand)
	writeJSON(w, http.StatusOK, map[string]any{
		"candidate": cand,
		"choices":   choices,
		"left":      g,
	})
}

func (s *Server) applyGameScrape(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "validation", "无效 id")
		return
	}
	var body struct {
		Candidate scrape.Candidate    `json:"candidate"`
		Choices   scrape.ApplyChoices `json:"choices"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	if body.Candidate.ID == "" || body.Candidate.Provider == "" {
		writeError(w, http.StatusBadRequest, "validation", "需要选中搜索结果")
		return
	}
	g, err := s.lib.GetGame(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "游戏不存在")
		return
	}

	s.mu.RLock()
	svc := scrape.New(s.db, s.cfg)
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	updated, err := svc.ApplyCandidate(ctx, g, body.Candidate, body.Choices)
	if err != nil {
		msg := err.Error()
		if e, ok := err.(*scrape.Error); ok {
			msg = e.Message
		}
		writeError(w, http.StatusBadGateway, "scrape", msg)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) browseGameMedia(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "validation", "无效 id")
		return
	}
	var body struct {
		Provider string `json:"provider"`
		Kind     string `json:"kind"`
		Query    string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	if strings.TrimSpace(body.Provider) == "" || strings.TrimSpace(body.Kind) == "" {
		writeError(w, http.StatusBadRequest, "validation", "需要 provider 与 kind")
		return
	}
	g, err := s.lib.GetGame(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "游戏不存在")
		return
	}

	s.mu.RLock()
	svc := scrape.New(s.db, s.cfg)
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	result, err := svc.BrowseMedia(ctx, g, body.Provider, body.Kind, body.Query)
	if err != nil {
		msg := err.Error()
		if e, ok := err.(*scrape.Error); ok {
			msg = e.Message
		}
		writeError(w, http.StatusBadGateway, "scrape", msg)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) pickGameMedia(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "validation", "无效 id")
		return
	}
	var body struct {
		Provider string `json:"provider"`
		Kind     string `json:"kind"`
		URL      string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	if strings.TrimSpace(body.Kind) == "" || strings.TrimSpace(body.URL) == "" {
		writeError(w, http.StatusBadRequest, "validation", "需要 kind 与 url")
		return
	}
	g, err := s.lib.GetGame(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "游戏不存在")
		return
	}

	s.mu.RLock()
	svc := scrape.New(s.db, s.cfg)
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	updated, err := svc.PickMedia(ctx, g, body.Kind, body.URL, body.Provider)
	if err != nil {
		msg := err.Error()
		if e, ok := err.(*scrape.Error); ok {
			msg = e.Message
		}
		writeError(w, http.StatusBadGateway, "scrape", msg)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) uploadGameMedia(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "validation", "无效 id")
		return
	}
	kind := r.PathValue("kind")
	if _, err := library.NormalizeMediaKind(kind); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "不支持的媒体类型")
		return
	}
	g, err := s.lib.GetGame(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "游戏不存在")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请上传图片文件")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation", "缺少 file 字段")
		return
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 16<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation", "读取文件失败")
		return
	}
	ext := library.ExtFromFilename(hdr.Filename, hdr.Header.Get("Content-Type"))
	s.mu.RLock()
	dataRoot := s.cfg.Paths.Data
	s.mu.RUnlock()
	asset, err := s.lib.SaveMediaFile(dataRoot, g, kind, body, ext, "upload")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "保存媒体失败")
		return
	}
	updated, err := s.lib.GetGame(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "读取游戏失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"media": asset, "game": updated})
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
