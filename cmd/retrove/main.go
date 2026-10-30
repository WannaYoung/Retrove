package main

import (
	"log"
	"net/http"
	"os"

	"retrove/internal/api"
	"retrove/internal/config"
	"retrove/internal/platform"
	"retrove/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("ensure dirs: %v", err)
	}
	db, err := store.Open(cfg.SQLite.Path)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	platformsPath := platform.DefaultPath(cfg.Paths.Config)
	defs, err := platform.LoadFile(platformsPath)
	if err != nil {
		log.Fatalf("load platforms %s: %v", platformsPath, err)
	}
	if err := platform.Sync(db, defs); err != nil {
		log.Fatalf("sync platforms: %v", err)
	}

	srv := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: api.New(cfg, db).Handler(),
	}
	log.Printf("retrove listening on %s (roms=%s data=%s platforms=%d)", cfg.Server.Listen, cfg.Paths.Roms, cfg.Paths.Data, len(defs))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Println(err)
		os.Exit(1)
	}
}
