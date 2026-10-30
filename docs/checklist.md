# Retrove — Development Checklist

[中文](./开发清单.md)

> Phased implementation list. Done criteria sit after each item; tick boxes as you go.  
> Design: [development.md](./development.md).

**Stack:** Go backend · Vue3 + Naive UI · SQLite · Docker · v1 ScreenScraper + local dictionary · export to `/app/data/export`.  
**Identity:** Consoles = hash of ROM inside archive; arcade = zip hash; one file → one game; Chinese titles live in DB; dictionary files are import/backup only; match local first, then ScreenScraper.  
**Paths:** Container mounts `/app/config`, `/app/data`, `/app/roms`; docs in root `docs/`; local frontend/backend via root `Makefile`.

---

## Status legend

- `[ ]` Not started  
- `[~]` In progress  
- `[x]` Done  
- `[-]` Deferred / won’t do (note why)

---

## Phase 0 — Repo & skeleton

- [x] Source at repo root: `cmd/retrove/`, `internal/`, `web/`, `go.mod`; `app/` only `config/`, `data/`, `roms/` (runtime mounts, no source)
- [x] Go module; root `Makefile` for `lint` / `fmt` / `test` / local run
- [x] Makefile targets: `make dev-backend`, `make dev-frontend` (split), `make build`, `make docker-up`
- [x] Vue3 + Vite + TS + Naive UI scaffold; proxy to backend API
- [x] Draft `docker-compose.yml` (mounts `/app/config`, `/app/data`, `/app/roms`)
- [x] Sample `app/config/config.example.yaml` (no secrets)
- [x] `.gitignore`: `app/data/`, secrets, `node_modules/`, build artifacts (`roms/` as needed)
- [x] README links into `docs/` and documents `make dev-backend` / `make dev-frontend`
- [x] Frontend multi-theme (`main.css`) + zh/en; language follows browser by default, choice persisted in localStorage

**Done when:** Root `make dev-backend` / `make dev-frontend` work; `GET /api/v1/health` returns ok.

Also done: backend `:5080`, frontend `:5180`; single-user register/login (password ≥8, confirm; hide register when a user exists).

---

## Phase 1 — Config, SQLite, platform map

- [x] Open SQLite; WAL + busy_timeout (`users` / `sessions` landed)
- [x] Config load/validation; startup `EnsureDirs` for data/media/export/cache
- [x] Empty database creates the current schema on startup (no incremental migrations)
- [x] Tables: `platforms`, `games`, `rom_files`, `aliases`, `media_assets`, `scrape_cache`, `jobs`
- [x] `rom_files`: unique `path`; partial unique `(sha1, size)`; indexes on `md5`/`crc32`
- [x] `/app/config/platforms.yaml`: folder ↔ platform; Arcade `hash_mode=archive` + `rename_policy=deny`
- [x] Seed platforms covering common `app/roms` folders; Sync on boot; `GET /api/v1/platforms` (auth)

**Done when:** Empty DB migrates; restart keeps schema; platforms API works.

---

## Phase 2 — Scan & hash identity

- [x] Walk ROM root; assign by platform `directory`; filter extensions
- [x] Consoles: `hash_mode=content` → CRC32/MD5/SHA1 of primary ROM inside zip; arcade/archive → zip itself; skip rehash if size+mtime unchanged
- [x] Hash hit → update `path`/`filename` (rename detection)
- [x] Hash miss → insert `rom_files` + new `game` (one file, one game)
- [x] Paths not seen this pass → `missing=true`
- [x] Same hash at another still-present path → `duplicate` in report
- [x] `POST /api/v1/jobs/scan`, `GET /api/v1/jobs`, `GET /api/v1/jobs/{id}`
- [x] Scan report: added / renamed / missing / duplicate / skipped / scanned

**Done when:** Unit tests cover rename merge and zip content hash; API can start async scan.

---

## Phase 3 — Pegasus import + local dictionary

- [x] Parse `metadata.pegasus.txt` (collection / game / file / description, …)
- [x] Align `file:` to `rom_files`; write `games.title_zh`, description, etc.
- [x] Import writes `aliases` (name / filename); dictionary file backup export later
- [x] Game list/detail queries (platform + ZH/EN/filename search)
- [x] Import per platform folder; do not merge `SFC` and `SFC hack`
- [x] Pegasus import overwrites Chinese title, description, and developer; copies `media/{ROM stem}/boxFront.*`, `logo.*`, and `video.*`; the detail panel can play and replace the preview video
- [x] `PATCH /api/v1/games/{id}` writes Chinese title, description, and developer back to Pegasus metadata; host platforms also rename the ROM and media folder; arcade does not rename
- [x] Replacing box / logo in the detail panel writes the Pegasus media folder; `POST /api/v1/jobs/import-pegasus`
- [x] Detail sidebar editable (title / description / media; Pegasus write-back)

**Done when:** After importing an FC Pegasus tree, Chinese titles, blurbs, box art, logos, and preview videos match the files; a later import replaces those fields.

---

## Phase 4 — Core admin UI

- [x] Shell: narrow sidebar + top bar (language/theme); iiSU-like cover grid, not a dashboard card wall
- [x] Overview: platforms/games/ROMs/missing; quick scan & import
- [x] Platform library: filter + search + cover grid
- [x] Game detail: sidebar edit ZH/EN title/description/lock
- [x] Scan page: scan ROMs, import Pegasus, show report
- [x] Settings: read-only paths + appearance (language/theme); SS credentials deferred to Phase 5
- [x] Jobs page: list + status

**Done when:** Imported library browsable without scraping; Chinese titles editable by hand.

---

## Phase 5 — Multi-source scrape (ScreenScraper + IGDB + SteamGridDB)

- [x] SS client: auth, rate limit, error classes (network / quota / not found)
- [x] Query by content hash (arcade zip hash) → filename fallback; existing fields and media are filled only when empty
- [x] Fill English + media on `games` (unlocked fields) + `scrape_cache`
- [x] Download media to `/app/data/media/...`, write `media_assets`
- [x] After scrape, apply local dictionary to fill/correct `title_zh` (respect lock)
- [x] Jobs: `scrape_missing` / `scrape_platform` / `scrape_one`
- [x] Scrape center UI: trigger, progress, retry failures
- [x] Arcade: metadata/media only; never rename set filenames
- [x] IGDB: by-name English metadata/covers (Twitch Client ID/Secret)
- [x] SteamGridDB: by-name portrait covers (API Key)
- [x] Settings: provider status on first level; credentials in nested dialog

**Done when:** Any configured source can scrape; batch prefers IGDB (English name + platform); SS fills remaining gaps; SGDB fills missing media only; clear job errors on offline/quota.

---

## Phase 6 — Wire the match pipeline

- [ ] Unified match service: in DB → local Pegasus/dictionary → ScreenScraper → unidentified
- [ ] Optional “auto match” after scan
- [ ] Unidentified list + manual link to existing / new game
- [ ] Bulk “apply dictionary”

**Done when:** Doc match order matches code + UI; rescan after rename keeps scrape results.

---

## Phase 7 — Export (Pegasus first)

- [ ] Exporter interface: `Export(ctx, target, opts) → report`
- [ ] Write Pegasus to `/app/data/export/pegasus/<platform>/metadata.pegasus.txt`
- [ ] Media copy or relative-path strategy (document one, implement it)
- [ ] Export job + API + export page
- [ ] Round-trip with import: spot-check fields and Chinese titles

**Done when:** DB-exported Pegasus is understandable by a frontend or re-import; files under `/app/data/export` (host `app/data/export`).

---

## Phase 8 — More export adapters

- [ ] EmuELEC / EmulationStation: `gamelist.xml` + media conventions
- [ ] iisu: same
- [ ] Cocoon: same
- [ ] Extend `platforms.yaml` with each target’s system id / folder names
- [ ] Export page: multi-target selection

**Done when:** Each target has at least one sample-platform export usable on device or a mock tree.

---

## Phase 9 — Polish & quality

- [ ] OpenAPI / route table documented
- [ ] Unit tests for critical paths: hash rename merge, lock vs scrape, dictionary priority
- [ ] Log levels; redact secrets
- [ ] Backup notes: `config/retrove.db` + `config` + (optional) `data/media`
- [x] Multi-stage image; SPA embedded in Go; container UI `:5180`, API `:5080`; entrypoint applies `PUID`/`PGID`/`TZ` and seeds `platforms.yaml` / `config.yaml` when missing
- [x] In-process job workers (scan / scrape / import) so HTTP stays responsive

**Done when:** Following [deployment.md](./deployment.md), a from-scratch deploy can demo “scan → import/dictionary → scrape → export”.

---

## Phase 10 — Later (explicitly not v1)

- [x] IGDB / SteamGridDB second scrapers (folded into Phase 5)
- [ ] LLM rename review queue (default no disk rename)
- [ ] Multi-user access control
- [ ] Optional PostgreSQL backend
- [ ] Bulk physical rename tool (single-game detail save already renames host ROMs and media folders; arcade `rename_policy: deny` still does not rename)

---

## Suggested order (follow strictly while building)

```text
Phase 0 → 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9
         (10 only after v1 acceptance)
```

Note: Phase 4 can overlap late Phase 3 (frontend on mocks); Phase 5 needs real SS credentials.

---

## v1 acceptance (demo path)

1. `docker compose up`  
2. Mount an existing Chinese Pegasus ROM library  
3. Scan succeeds; rename + rescan keeps identity  
4. Import Pegasus metadata; Chinese titles, blurbs, box art, logos, and preview videos visible  
5. Translate to English, then scrape; existing Chinese titles and covers stay  
6. Edit a Chinese title in the detail panel; ROM filename, metadata, and media folder update together  
7. Export Pegasus to `app/data/export` (container `/app/data/export`)  
8. Admin UI can do all of the above without code changes  

When all pass, close v1, then open Phase 8/10.
