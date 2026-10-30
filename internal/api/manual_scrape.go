package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"retrove/internal/library"
	"retrove/internal/pegasus"
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
	if err := s.syncPegasusFields(g, updated); err != nil {
		revert := library.UpdateGameInput{
			TitleZH:     &g.TitleZH,
			Description: &g.Description,
			Developer:   &g.Developer,
		}
		_, _ = s.lib.UpdateGame(id, revert)
		var conflict *pegasus.ConflictError
		if errors.As(err, &conflict) {
			writeError(w, http.StatusConflict, "conflict", conflict.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	updated, err = s.lib.GetGame(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "读取游戏失败")
		return
	}
	for _, kind := range []string{"box", "logo"} {
		if gameMediaPath(g, kind) == gameMediaPath(updated, kind) {
			continue
		}
		if err := s.mirrorSavedPegasusMedia(updated, kind); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, updated)
}

func gameMediaPath(g library.Game, kind string) string {
	for _, m := range g.Media {
		if m.Kind == kind && strings.TrimSpace(m.Path) != "" {
			return m.Path
		}
	}
	if kind == "box" {
		return strings.TrimSpace(g.CoverPath)
	}
	return ""
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
	if err := s.mirrorSavedPegasusMedia(updated, body.Kind); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) mirrorPegasusMedia(g library.Game, kind string, body []byte, ext string) error {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "box" && kind != "logo" && kind != "video" {
		return nil
	}
	if strings.TrimSpace(g.Filename) == "" {
		return nil
	}
	var dir string
	if err := s.db.QueryRow(`SELECT directory FROM platforms WHERE id=?`, g.PlatformID).Scan(&dir); err != nil {
		return err
	}
	s.mu.RLock()
	romsRoot := s.cfg.Paths.Roms
	s.mu.RUnlock()
	return pegasus.MirrorMedia(romsRoot, dir, g.Filename, kind, body, ext)
}

func (s *Server) mirrorSavedPegasusMedia(g library.Game, kind string) error {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "wheel" {
		kind = "logo"
	}
	var rel string
	for _, m := range g.Media {
		if m.Kind == kind {
			rel = m.Path
			break
		}
	}
	if rel == "" {
		return nil
	}
	s.mu.RLock()
	dataRoot := s.cfg.Paths.Data
	s.mu.RUnlock()
	body, err := os.ReadFile(filepath.Join(dataRoot, "media", filepath.FromSlash(rel)))
	if err != nil {
		return err
	}
	return s.mirrorPegasusMedia(g, kind, body, filepath.Ext(rel))
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
	norm, err := library.NormalizeMediaKind(kind)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation", "不支持的媒体类型")
		return
	}
	kind = norm
	g, err := s.lib.GetGame(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "游戏不存在")
		return
	}
	limit := int64(16 << 20)
	if library.IsVideoKind(kind) {
		limit = 64 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := r.ParseMultipartForm(limit); err != nil {
		if library.IsVideoKind(kind) {
			writeError(w, http.StatusBadRequest, "validation", "请上传视频文件")
			return
		}
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
	if err := s.mirrorPegasusMedia(g, kind, body, ext); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
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
