package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := EnsureSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// EnsureSchema creates the current tables. Empty databases are the supported path.
func EnsureSchema(db *sql.DB) error {
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	return nil
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL,
  expires_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS platforms (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  directory TEXT NOT NULL UNIQUE,
  extensions TEXT NOT NULL DEFAULT '',
  hash_mode TEXT NOT NULL DEFAULT 'content',
  rename_policy TEXT NOT NULL DEFAULT 'allow',
  screenscraper_id INTEGER NOT NULL DEFAULT 0,
  igdb_id INTEGER NOT NULL DEFAULT 0,
  scrape_as TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS games (
  id INTEGER PRIMARY KEY,
  platform_id TEXT NOT NULL REFERENCES platforms(id),
  title_zh TEXT NOT NULL DEFAULT '',
  title_en TEXT NOT NULL DEFAULT '',
  title_ja TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  developer TEXT NOT NULL DEFAULT '',
  release_year INTEGER,
  title_zh_locked INTEGER NOT NULL DEFAULT 1,
  description_locked INTEGER NOT NULL DEFAULT 1,
  scrape_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_games_platform ON games(platform_id);
CREATE INDEX IF NOT EXISTS idx_games_title_zh ON games(title_zh);

CREATE TABLE IF NOT EXISTS rom_files (
  id INTEGER PRIMARY KEY,
  platform_id TEXT NOT NULL REFERENCES platforms(id),
  game_id INTEGER REFERENCES games(id),
  path TEXT NOT NULL UNIQUE,
  filename TEXT NOT NULL,
  size INTEGER NOT NULL DEFAULT 0,
  mtime INTEGER NOT NULL DEFAULT 0,
  crc32 TEXT NOT NULL DEFAULT '',
  md5 TEXT NOT NULL DEFAULT '',
  sha1 TEXT NOT NULL DEFAULT '',
  missing INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_rom_files_sha1_size ON rom_files(sha1, size) WHERE sha1 != '';
CREATE INDEX IF NOT EXISTS idx_rom_files_md5 ON rom_files(md5);
CREATE INDEX IF NOT EXISTS idx_rom_files_crc32 ON rom_files(crc32);
CREATE INDEX IF NOT EXISTS idx_rom_files_platform ON rom_files(platform_id);
CREATE INDEX IF NOT EXISTS idx_rom_files_game ON rom_files(game_id);

CREATE TABLE IF NOT EXISTS aliases (
  id INTEGER PRIMARY KEY,
  platform_id TEXT NOT NULL REFERENCES platforms(id),
  game_id INTEGER REFERENCES games(id),
  kind TEXT NOT NULL DEFAULT 'name',
  value TEXT NOT NULL,
  UNIQUE(platform_id, kind, value)
);

CREATE INDEX IF NOT EXISTS idx_aliases_game ON aliases(game_id);

CREATE TABLE IF NOT EXISTS media_assets (
  id INTEGER PRIMARY KEY,
  game_id INTEGER NOT NULL REFERENCES games(id),
  kind TEXT NOT NULL,
  path TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT '',
  UNIQUE(game_id, kind)
);

CREATE TABLE IF NOT EXISTS scrape_cache (
  id INTEGER PRIMARY KEY,
  key TEXT NOT NULL UNIQUE,
  source TEXT NOT NULL,
  payload TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS jobs (
  id INTEGER PRIMARY KEY,
  type TEXT NOT NULL,
  status TEXT NOT NULL,
  progress REAL NOT NULL DEFAULT 0,
  message TEXT NOT NULL DEFAULT '',
  report TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
`
