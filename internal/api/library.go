package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"retrove/internal/jobs"
	"retrove/internal/library"
	"retrove/internal/pegasus"
)

func (s *Server) libraryStats(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	st, err := s.lib.Stats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "读取统计失败")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) listGames(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	items, err := s.lib.ListGames(q.Get("platform"), q.Get("q"), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "读取游戏失败")
		return
	}
	if items == nil {
		items = []library.Game{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) getGame(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation", "无效 id")
		return
	}
	g, err := s.lib.GetGame(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "游戏不存在")
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) updateGame(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation", "无效 id")
		return
	}
	var in library.UpdateGameInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	g, err := s.lib.UpdateGame(id, in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "更新失败")
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) startImportPegasus(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	job, err := s.jobs.Create("import_pegasus")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "创建任务失败")
		return
	}
	go s.runImportPegasus(job.ID)
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) runImportPegasus(jobID int64) {
	_ = s.jobs.Update(jobID, jobs.StatusRunning, 0, "导入 Pegasus", nil)
	rep, err := pegasus.ImportAll(s.db, s.cfg.Paths.Roms)
	if err != nil {
		_ = s.jobs.Update(jobID, jobs.StatusFailed, 1, err.Error(), nil)
		return
	}
	_ = s.jobs.Update(jobID, jobs.StatusSucceeded, 1, "导入完成", rep)
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"paths": map[string]string{
			"roms":   s.cfg.Paths.Roms,
			"data":   s.cfg.Paths.Data,
			"config": s.cfg.Paths.Config,
			"sqlite": s.cfg.SQLite.Path,
			"export": s.cfg.Export.Root,
		},
		"providers": map[string]any{
			"screenscraper": map[string]any{
				"configured": s.cfg.ScreenScraperConfigured(),
				"user_set":   s.cfg.ScreenScraper.UserSS != "",
				"dev_id":     s.cfg.ScreenScraper.DevID,
				"user_ss":    s.cfg.ScreenScraper.UserSS,
				"softname":   s.cfg.ScreenScraper.Softname,
			},
			"igdb": map[string]any{
				"configured": s.cfg.IGDBConfigured(),
				"client_id":  s.cfg.IGDB.ClientID,
			},
			"steamgriddb": map[string]any{
				"configured":  s.cfg.SteamGridDBConfigured(),
				"api_key_set": s.cfg.SteamGridDBConfigured(),
			},
			"llm": map[string]any{
				"configured":  s.cfg.LLMConfigured(),
				"base_url":    s.cfg.LLM.BaseURL,
				"model":       s.cfg.LLM.Model,
				"api_key_set": strings.TrimSpace(s.cfg.LLM.APIKey) != "",
			},
		},
		"screenscraper_configured": s.cfg.ScreenScraperConfigured(),
		"screenscraper_user_set":   s.cfg.ScreenScraper.UserSS != "",
		"igdb_configured":          s.cfg.IGDBConfigured(),
		"steamgriddb_configured":   s.cfg.SteamGridDBConfigured(),
		"any_scrape_configured":    s.cfg.AnyScrapeSourceConfigured(),
		"llm_configured":           s.cfg.LLMConfigured(),
		"allow_rom_rename":         s.cfg.Security.AllowROMRename,
	})
}
