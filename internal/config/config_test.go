package config

import (
	"path/filepath"
	"testing"
)

func TestValidateAndEnsureDirs(t *testing.T) {
	t.Setenv("RETROVE_CONFIG", filepath.Join("..", "..", "app", "config", "config.example.yaml"))
	t.Setenv("RETROVE_DATA", t.TempDir())
	t.Setenv("RETROVE_ROMS", t.TempDir())
	t.Setenv("RETROVE_CONFIG_DIR", t.TempDir())
	t.Setenv("RETROVE_SQLITE", filepath.Join(t.TempDir(), "db", "retrove.db"))
	t.Setenv("RETROVE_EXPORT", filepath.Join(t.TempDir(), "export"))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Listen != ":5080" {
		t.Fatalf("%s", cfg.Server.Listen)
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
}
