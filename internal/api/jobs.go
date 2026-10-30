package api

import (
	"net/http"
	"strconv"

	"retrove/internal/jobs"
	"retrove/internal/scan"
)

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := s.currentUser(r); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "未登录")
		return false
	}
	return true
}

func (s *Server) startScan(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	job, err := s.jobs.Create("scan")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "创建任务失败")
		return
	}
	go s.runScan(job.ID)
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) runScan(jobID int64) {
	_ = s.jobs.Update(jobID, jobs.StatusRunning, 0, "扫描中", nil)
	rep, err := s.scan.Run(scan.Options{
		RomsRoot:       s.cfg.Paths.Roms,
		FollowSymlinks: s.cfg.Scan.FollowSymlinks,
	}, func(done, total int, msg string) {
		progress := 0.0
		if total > 0 {
			progress = float64(done) / float64(total)
		}
		_ = s.jobs.Update(jobID, jobs.StatusRunning, progress, msg, nil)
	})
	if err != nil {
		_ = s.jobs.Update(jobID, jobs.StatusFailed, 1, err.Error(), nil)
		return
	}
	_ = s.jobs.Update(jobID, jobs.StatusSucceeded, 1, "扫描完成", rep)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation", "无效任务 id")
		return
	}
	job, err := s.jobs.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "任务不存在")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	items, err := s.jobs.List(50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "读取任务失败")
		return
	}
	if items == nil {
		items = []jobs.Job{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
