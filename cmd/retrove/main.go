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

const uiListen = ":5180"

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

	srv := api.New(cfg, db)
	apiHandler := srv.Handler()

	if admin, ok := srv.AdminHandler(); ok {
		go func() {
			log.Printf("retrove api on %s (roms=%s data=%s platforms=%d)", cfg.Server.Listen, cfg.Paths.Roms, cfg.Paths.Data, len(defs))
			if err := http.ListenAndServe(cfg.Server.Listen, apiHandler); err != nil {
				log.Println(err)
				os.Exit(1)
			}
		}()
		log.Printf("retrove ui on %s (/api -> %s)", uiListen, cfg.Server.Listen)
		if err := http.ListenAndServe(uiListen, admin); err != nil {
			log.Println(err)
			os.Exit(1)
		}
		return
	}

	log.Printf("retrove listening on %s (roms=%s data=%s platforms=%d)", cfg.Server.Listen, cfg.Paths.Roms, cfg.Paths.Data, len(defs))
	if err := http.ListenAndServe(cfg.Server.Listen, apiHandler); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
