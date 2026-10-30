package api

import (
	"net/http"

	"retrove/internal/platform"
)

func (s *Server) listPlatforms(w http.ResponseWriter, r *http.Request) {
	if !s.requireUser(w, r) {
		return
	}
	items, err := platform.List(s.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "读取平台失败")
		return
	}
	if items == nil {
		items = []platform.Definition{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
