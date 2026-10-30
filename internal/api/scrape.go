package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"retrove/internal/config"
	"retrove/internal/jobs"
	"retrove/internal/scrape"
)

func (s *Server) startScrapeMissing(w http.ResponseWriter, r *http.Request) {
	s.startScrapeJob(w, r, scrape.Options{Mode: scrape.ModeMissing, Fields: scrape.DefaultFields()}, "scrape_missing")
}

func (s *Server) startScrapePlatform(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PlatformID  string          `json:"platform_id"`
		OnlyMissing bool            `json:"only_missing"`
		WriteMode   scrape.WriteMode `json:"write_mode"`
		Fields      scrape.Fields   `json:"fields"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if strings.TrimSpace(body.PlatformID) == "" {
		writeError(w, http.StatusBadRequest, "validation", "需要 platform_id")
		return
	}
	s.startScrapeJob(w, r, scrape.Options{
		Mode:        scrape.ModePlatform,
		PlatformID:  body.PlatformID,
		OnlyMissing: body.OnlyMissing,
		WriteMode:   body.WriteMode,
		Fields:      body.Fields,
	}, "scrape_platform")
}

func (s *Server) startScrapeOne(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GameID    int64            `json:"game_id"`
		WriteMode scrape.WriteMode `json:"write_mode"`
		Fields    scrape.Fields    `json:"fields"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.GameID <= 0 {
		writeError(w, http.StatusBadRequest, "validation", "需要 game_id")
		return
	}
	s.startScrapeJob(w, r, scrape.Options{
		Mode:      scrape.ModeOne,
		GameID:    body.GameID,
		WriteMode: body.WriteMode,
		Fields:    body.Fields,
	}, "scrape_one")
}

func (s *Server) startScrape(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PlatformID  string           `json:"platform_id"`
		OnlyMissing bool             `json:"only_missing"`
		WriteMode   scrape.WriteMode `json:"write_mode"`
		GameID      int64            `json:"game_id"`
		Fields      scrape.Fields    `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	if body.GameID > 0 {
		s.startScrapeJob(w, r, scrape.Options{
			Mode:      scrape.ModeOne,
			GameID:    body.GameID,
			WriteMode: body.WriteMode,
			Fields:    body.Fields,
		}, "scrape_one")
		return
	}
	if strings.TrimSpace(body.PlatformID) == "" {
		writeError(w, http.StatusBadRequest, "validation", "需要 platform_id")
		return
	}
	s.startScrapeJob(w, r, scrape.Options{
		Mode:        scrape.ModePlatform,
		PlatformID:  body.PlatformID,
		OnlyMissing: body.OnlyMissing,
		WriteMode:   body.WriteMode,
		Fields:      body.Fields,
	}, "scrape")
}

func (s *Server) startScrapeJob(w http.ResponseWriter, r *http.Request, opts scrape.Options, jobType string) {
	if !s.requireUser(w, r) {
		return
	}
	s.mu.RLock()
	ok := s.cfg.AnyScrapeSourceConfigured()
	s.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusBadRequest, "config", "未配置任何刮削源")
		return
	}
	job, err := s.jobs.Create(jobType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "创建任务失败")
		return
	}
	go s.runScrape(job.ID, opts)
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) runScrape(jobID int64, opts scrape.Options) {
	_ = s.jobs.Update(jobID, jobs.StatusRunning, 0, "刮削中", nil)
	s.mu.RLock()
	svc := scrape.New(s.db, s.cfg)
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	rep, err := svc.Run(ctx, opts, func(done, total int, msg string) {
		progress := 0.0
		if total > 0 {
			progress = float64(done) / float64(total)
		}
		_ = s.jobs.Update(jobID, jobs.StatusRunning, progress, msg, nil)
	})
	if err != nil {
		_ = s.jobs.Update(jobID, jobs.StatusFailed, 1, err.Error(), rep)
		return
	}
	_ = s.jobs.Update(jobID, jobs.StatusSucceeded, 1, "刮削完成", rep)
}

func (s *Server) updateScreenScraper(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	var in config.ScreenScraperInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	s.mu.Lock()
	s.cfg.ApplyScreenScraper(in)
	err := s.cfg.Save()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "保存配置失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.providerStatusLocked())
}

func (s *Server) updateIGDB(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	var in config.IGDBInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	s.mu.Lock()
	s.cfg.ApplyIGDB(in)
	err := s.cfg.Save()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "保存配置失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.providerStatusLocked())
}

func (s *Server) updateSteamGridDB(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	var in config.SteamGridDBInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	s.mu.Lock()
	s.cfg.ApplySteamGridDB(in)
	err := s.cfg.Save()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "保存配置失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.providerStatusLocked())
}

func (s *Server) providerStatusLocked() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]any{
		"screenscraper_configured": s.cfg.ScreenScraperConfigured(),
		"screenscraper_user_set":   s.cfg.ScreenScraper.UserSS != "",
		"igdb_configured":          s.cfg.IGDBConfigured(),
		"steamgriddb_configured":   s.cfg.SteamGridDBConfigured(),
		"any_configured":           s.cfg.AnyScrapeSourceConfigured(),
	}
}

func (s *Server) serveMedia(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	rel := strings.TrimPrefix(r.PathValue("path"), "/")
	rel = filepath.ToSlash(rel)
	abs, err := scrape.MediaAbs(s.cfg.Paths.Data, rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation", "无效路径")
		return
	}
	http.ServeFile(w, r, abs)
}

func (s *Server) scrapeStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	missing, _ := s.lib.MissingMediaCount()
	st := s.providerStatusLocked()
	st["missing_covers"] = missing
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) testScrapeProvider(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	var body struct {
		Provider string `json:"provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Provider) == "" {
		writeError(w, http.StatusBadRequest, "validation", "需要 provider")
		return
	}
	s.mu.RLock()
	svc := scrape.New(s.db, s.cfg)
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := svc.TestProvider(ctx, body.Provider); err != nil {
		msg := err.Error()
		if e, ok := err.(*scrape.Error); ok {
			msg = e.Message
			if e.Cause != nil {
				msg = e.Message + ": " + e.Cause.Error()
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":       false,
			"provider": body.Provider,
			"message":  msg,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"provider": body.Provider,
		"message":  "ok",
	})
}
