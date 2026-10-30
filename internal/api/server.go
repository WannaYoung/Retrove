package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"retrove/internal/auth"
	"retrove/internal/config"
	"retrove/internal/jobs"
	"retrove/internal/library"
	"retrove/internal/scan"
	"retrove/internal/ui"
)

type Server struct {
	cfg  config.Config
	db   *sql.DB
	auth *auth.Service
	jobs *jobs.Service
	scan *scan.Service
	lib  *library.Service
	mux  *http.ServeMux
	mu   sync.RWMutex
}

func New(cfg config.Config, db *sql.DB) *Server {
	s := &Server{
		cfg:  cfg,
		db:   db,
		auth: auth.New(db),
		jobs: jobs.New(db),
		scan: scan.New(db),
		lib:  library.New(db),
		mux:  http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /api/v1/health", s.health)
	s.mux.HandleFunc("GET /api/v1/auth/status", s.authStatus)
	s.mux.HandleFunc("POST /api/v1/auth/register", s.register)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.login)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	s.mux.HandleFunc("GET /api/v1/auth/me", s.me)
	s.mux.HandleFunc("GET /api/v1/platforms", s.listPlatforms)
	s.mux.HandleFunc("GET /api/v1/library/stats", s.libraryStats)
	s.mux.HandleFunc("GET /api/v1/games", s.listGames)
	s.mux.HandleFunc("GET /api/v1/games/{id}", s.getGame)
	s.mux.HandleFunc("PATCH /api/v1/games/{id}", s.updateGame)
	s.mux.HandleFunc("POST /api/v1/games/{id}/media/{kind}", s.uploadGameMedia)
	s.mux.HandleFunc("POST /api/v1/games/{id}/scrape/search", s.searchGameScrape)
	s.mux.HandleFunc("POST /api/v1/games/{id}/scrape/preview", s.previewGameScrape)
	s.mux.HandleFunc("POST /api/v1/games/{id}/scrape/apply", s.applyGameScrape)
	s.mux.HandleFunc("POST /api/v1/games/{id}/scrape/media/browse", s.browseGameMedia)
	s.mux.HandleFunc("POST /api/v1/games/{id}/scrape/media/pick", s.pickGameMedia)
	s.mux.HandleFunc("GET /api/v1/settings", s.getSettings)
	s.mux.HandleFunc("PATCH /api/v1/settings/screenscraper", s.updateScreenScraper)
	s.mux.HandleFunc("PATCH /api/v1/settings/igdb", s.updateIGDB)
	s.mux.HandleFunc("PATCH /api/v1/settings/steamgriddb", s.updateSteamGridDB)
	s.mux.HandleFunc("PATCH /api/v1/settings/llm", s.updateLLM)
	s.mux.HandleFunc("GET /api/v1/llm/models", s.listLLMModels)
	s.mux.HandleFunc("POST /api/v1/llm/test", s.testLLM)
	s.mux.HandleFunc("GET /api/v1/scrape/status", s.scrapeStatus)
	s.mux.HandleFunc("POST /api/v1/scrape/test", s.testScrapeProvider)
	s.mux.HandleFunc("POST /api/v1/jobs/scan", s.startScan)
	s.mux.HandleFunc("POST /api/v1/jobs/import-pegasus", s.startImportPegasus)
	s.mux.HandleFunc("POST /api/v1/jobs/scrape", s.startScrape)
	s.mux.HandleFunc("POST /api/v1/jobs/scrape-missing", s.startScrapeMissing)
	s.mux.HandleFunc("POST /api/v1/jobs/scrape-platform", s.startScrapePlatform)
	s.mux.HandleFunc("POST /api/v1/jobs/scrape-one", s.startScrapeOne)
	s.mux.HandleFunc("POST /api/v1/jobs/translate-platform", s.startTranslatePlatform)
	s.mux.HandleFunc("GET /api/v1/jobs/{id}", s.getJob)
	s.mux.HandleFunc("GET /api/v1/jobs", s.listJobs)
	s.mux.HandleFunc("GET /api/v1/media/{path...}", s.serveMedia)
	s.mux.HandleFunc("GET /{$}", s.root)
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

// AdminHandler serves the embedded SPA on :5180 and forwards /api to the API mux
// (same layout as local Vite → :5080).
func (s *Server) AdminHandler() (http.Handler, bool) {
	uiFS, ok := ui.Dist()
	if !ok {
		return nil, false
	}
	spa := ui.Handler(uiFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			s.mux.ServeHTTP(w, r)
			return
		}
		spa.ServeHTTP(w, r)
	}), true
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) root(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("Retrove API\n健康检查: /api/v1/health\n管理台开发地址: http://127.0.0.1:5180\n"))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
