package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"retrove/internal/config"
	"retrove/internal/jobs"
	"retrove/internal/translate"
)

func (s *Server) updateLLM(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	var in config.LLMInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation", "请求格式不正确")
		return
	}
	s.mu.Lock()
	s.cfg.ApplyLLM(in)
	err := s.cfg.Save()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "保存配置失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.llmStatusLocked())
}

func (s *Server) llmStatusLocked() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]any{
		"configured":  s.cfg.LLMConfigured(),
		"base_url":    s.cfg.LLM.BaseURL,
		"model":       s.cfg.LLM.Model,
		"api_key_set": strings.TrimSpace(s.cfg.LLM.APIKey) != "",
	}
}

func (s *Server) listLLMModels(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	s.mu.RLock()
	base := s.cfg.LLM.BaseURL
	key := s.cfg.LLM.APIKey
	model := s.cfg.LLM.Model
	s.mu.RUnlock()

	qBase := strings.TrimSpace(r.URL.Query().Get("base_url"))
	qKey := strings.TrimSpace(r.URL.Query().Get("api_key"))
	if qBase != "" {
		base = qBase
	}
	if qKey != "" {
		key = qKey
	}
	if strings.TrimSpace(base) == "" || strings.TrimSpace(key) == "" {
		writeError(w, http.StatusBadRequest, "config", "请先填写 Base URL 与 API Key")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	client := translate.NewClient(base, key, model)
	items, err := client.ListModels(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "llm", err.Error())
		return
	}
	if items == nil {
		items = []translate.ModelInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "model": model})
}

func (s *Server) testLLM(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	var body struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		Model   string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	s.mu.RLock()
	base := s.cfg.LLM.BaseURL
	key := s.cfg.LLM.APIKey
	model := s.cfg.LLM.Model
	s.mu.RUnlock()
	if strings.TrimSpace(body.BaseURL) != "" {
		base = body.BaseURL
	}
	if strings.TrimSpace(body.APIKey) != "" {
		key = body.APIKey
	}
	if strings.TrimSpace(body.Model) != "" {
		model = body.Model
	}
	if strings.TrimSpace(base) == "" || strings.TrimSpace(key) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "请先填写 Base URL 与 API Key"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	client := translate.NewClient(base, key, model)
	if err := client.Ping(ctx); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "ok"})
}

func (s *Server) startTranslatePlatform(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	var body struct {
		PlatformID string `json:"platform_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if strings.TrimSpace(body.PlatformID) == "" {
		writeError(w, http.StatusBadRequest, "validation", "需要 platform_id")
		return
	}
	s.mu.RLock()
	ok := s.cfg.LLMConfigured()
	s.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusBadRequest, "config", "未配置 LLM（需要 Base URL、API Key 与模型）")
		return
	}
	job, err := s.jobs.Create("translate_platform")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "创建任务失败")
		return
	}
	go s.runTranslate(job.ID, translate.Options{PlatformID: body.PlatformID})
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) runTranslate(jobID int64, opts translate.Options) {
	_ = s.jobs.Update(jobID, jobs.StatusRunning, 0, "翻译中", nil)
	s.mu.RLock()
	svc := translate.New(s.db, s.cfg)
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
	_ = s.jobs.Update(jobID, jobs.StatusSucceeded, 1, "翻译完成", rep)
}
