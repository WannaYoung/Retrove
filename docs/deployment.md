# Retrove — Deployment Guide

[中文](./部署指南.md)

> Local development and Docker deploy. Design: [development.md](./development.md). Tasks: [checklist.md](./checklist.md).  
> Directory layout, env vars, and recommended flows are fixed here; tweak commands as the project evolves, but keep mount conventions stable.

---

## 1. Directory layout

- **Docs:** repo root `docs/` (this file).  
- **Local:** root `Makefile` starts backend and frontend.  
- **Docker:** container working directory **`/app`**; persistent data under `/app/`.

Suggested tree:

`app/` holds **runtime data only** (config, DB, media, export)—no source. Source lives at repo root and is baked into the image; Compose mounts only the three dirs below into `/app`.

```text
retrove/                      # repo / Go module root
├── Makefile
├── go.mod
├── docs/
├── docker-compose.yml
├── cmd/retrove/              # backend entry
├── internal/                 # backend packages
├── web/                      # Vue3 admin
└── app/                      # runtime only
    ├── config/               # → /app/config
    ├── data/                 # → /app/data
    └── roms/                 # → /app/roms
```

Put the ROM library in **`app/roms/`** (often with Pegasus metadata already present), same class of runtime dir as config and data.

| Host | Container | Purpose |
|------|-----------|---------|
| `./app/config` | `/app/config` | Config, platform map, dictionaries, API secrets |
| `./app/data` | `/app/data` | SQLite, media, cache, **export** |
| `./app/roms` | `/app/roms` | ROM files (only this tree is scanned) |

Exports write to **`/app/data/export/<target>/`** (host: `app/data/export/`).

---

## 2. Configuration

### 2.1 `app/config/config.yaml` (shape)

```yaml
server:
  listen: ":5080"

paths:
  roms: /app/roms
  data: /app/data
  config: /app/config

sqlite:
  path: /app/data/retrove.db
  # WAL enabled via PRAGMA at startup

screenscraper:
  dev_id: ""
  dev_password: ""
  user_ss: ""
  user_password: ""
  softname: Retrove
  # Respect quotas; leave empty to disable scraping (rest of app still works)

scan:
  rehash_on_mtime_size_change: true
  follow_symlinks: false

export:
  root: /app/data/export
  # Default: do not overwrite metadata next to user ROMs

security:
  allow_rom_rename: false   # global physical rename switch; Arcade still deny per platform

log:
  level: info
```

Also see committed `app/config/config.example.yaml` for optional IGDB / SteamGridDB / LLM blocks.

### 2.2 Secrets

- Keep ScreenScraper (and other) credentials under `app/config/`—**never commit them**.  
- Use `config.yaml` + file permissions, and/or env overrides (document whichever you implement).

### 2.3 Environment variables (suggested)

| Variable | Meaning |
|----------|---------|
| `PUID` / `PGID` | UID/GID for mounted dirs (common on NAS) |
| `TZ` | e.g. `Asia/Shanghai` |
| `RETROVE_CONFIG` | Config path. Local default `./app/config/config.yaml`; container `/app/config/config.yaml` |
| `RETROVE_LISTEN` | Listen address; local and container both `:5080` |
| `RETROVE_ROMS` / `RETROVE_DATA` / `RETROVE_CONFIG_DIR` | Override yaml paths. `make backend` sets local `./app/...` |
| `RETROVE_SQLITE` / `RETROVE_EXPORT` | Override DB file and export root |
| `RETROVE_LOG_LEVEL` | Override log level |

---

## 3. Docker deploy (target shape)

### 3.1 Compose sketch

```yaml
services:
  retrove:
    image: retrove:local   # or build: .
    working_dir: /app
    ports:
      - "5080:5080"
    environment:
      TZ: Asia/Shanghai
      PUID: "1000"
      PGID: "1000"
      RETROVE_CONFIG: /app/config/config.yaml
    volumes:
      - ./app/config:/app/config
      - ./app/data:/app/data
      - ./app/roms:/app/roms
    restart: unless-stopped
```

Prefer one container: Go serves the API and runs the job queue in-process (goroutine workers). Frontend static assets via Go embed or reverse-proxy in the same image (pick one; prefer a single service port).

### 3.2 First boot

```bash
mkdir -p app/config/dictionaries app/data/media app/data/export app/data/cache
cp app/config/config.example.yaml app/config/config.yaml
# Edit ScreenScraper credentials, ports, etc.

docker compose up -d --build
# Browser: http://localhost:5080 (API). For admin UI during development use make + http://127.0.0.1:5180
```

### 3.3 Data & backup

| Item | Need |
|------|------|
| `app/data/retrove.db` | Required (metadata) |
| `app/config/` | Required (dictionaries & secrets) |
| `app/data/media/` | Recommended (re-scrapable but slow / quota-heavy) |
| `app/data/export/` | Optional (re-exportable) |
| `app/roms/` | Your assets; follow your own backup policy |

Restore: stop → restore `app/data` + `app/config` → start → rescan if paths need aligning.

### 3.4 Permissions (NAS)

- Host directory owners should match `PUID/PGID`.  
- Read-only ROMs: mount the `/app/roms` volume `:ro` and keep `allow_rom_rename: false`.

---

## 4. Local development (Makefile)

Day-to-day work uses the root **`Makefile`**; Docker is optional.

### 4.1 Dependencies

- Go (match `go.mod`)
- Node.js LTS (frontend)
- `make`
- Optional: Docker (image / near-prod checks)

### 4.2 Makefile targets

| Target | Role |
|--------|------|
| `make dev-backend` | Go API (`app/config/config.yaml`, local paths `./app/roms`, `./app/data`) |
| `make dev-frontend` | Vite in `web/` on 5180, `/api` → 5080 |
| `make build` | Backend binary + frontend static build |
| `make test` / `make lint` | Tests and checks |
| `make docker-up` | `docker compose up -d --build` |
| `make docker-down` | Stop compose |

First time:

```bash
mkdir -p app/config/dictionaries app/data/media app/data/export app/data/cache
cp app/config/config.example.yaml app/config/config.yaml
# Makefile overrides paths to ./app/roms, ./app/data, ./app/config via env

# Two terminals:
make dev-backend
make dev-frontend
```

Health: `curl -s http://127.0.0.1:5080/api/v1/health`  
UI: http://127.0.0.1:5180

### 4.3 Local vs Docker paths

| Item | Local `make` | Docker |
|------|--------------|--------|
| Config file | `./app/config/config.yaml` | `/app/config/config.yaml` |
| `paths.roms` | `./app/roms` | `/app/roms` |
| `paths.data` | `./app/data` | `/app/data` |
| `paths.config` | `./app/config` | `/app/config` |

Same yaml + env path overrides, or a `config.local.yaml` (never commit secrets). `Makefile` should export `RETROVE_CONFIG` to the local file.

### 4.4 SQLite debugging

- DB: `app/data/retrove.db` (container `/app/data/retrove.db`)  
- Tools: `sqlite3` CLI or a GUI (careful with live WAL while the service runs).  

```bash
sqlite3 app/data/retrove.db "PRAGMA journal_mode; PRAGMA busy_timeout;"
sqlite3 app/data/retrove.db "SELECT COUNT(*) FROM rom_files;"
```

Reset DB (drops metadata, keeps ROMs):

```bash
make docker-down   # if using compose
rm -f app/data/retrove.db app/data/retrove.db-*
# then make backend or make docker-up — migrations run again
```

### 4.5 Typical debug path

1. `make dev-*`, sample with 1–2 platforms under `app/roms/SFC`, etc.  
2. Trigger scan → confirm hashes in `rom_files`.  
3. **Rename one zip** → scan again → same `sha1` row updates path (no new row).  
4. Import that platform’s `metadata.pegasus.txt` → Chinese title on detail page.  
5. Configure a ScreenScraper test account → scrape one title → check `app/data/media` and UI cover.  
6. Export Pegasus → inspect `app/data/export/pegasus/...`.

### 4.6 Logging

- Dev: `log.level: debug`  
- Quota hits, hash cost, and job failures should be searchable.  
- Never log password fields.

---

## 5. ScreenScraper debugging tips

- Validate on a small sample first; don’t burn the whole-library quota.  
- On hash miss, try filename; confirm the CRC exists on the ScreenScraper site.  
- Classify failures: auth / quota / not found / network; UI and job detail should distinguish them.  
- `scrape_cache` hits should skip re-download (renames must not re-pull the same media unless force-refresh).

---

## 6. Export acceptance

| Target | Expected path (example) |
|--------|-------------------------|
| Pegasus | `app/data/export/pegasus/<platform>/metadata.pegasus.txt` |
| EmuELEC | `app/data/export/emuelec/...` |
| Tianma | `app/data/export/tianma/...` |
| iisu | `app/data/export/iisu/...` |
| Cocoon | `app/data/export/cocoon/...` |

In-container prefixes are all `/app/data/export/...`.

Check: Chinese `game` names, relative file paths, media present; Arcade filenames not silently renamed.

---

## 7. FAQ

**Scan is slow**  
First pass hashes everything; later passes skip via size+mtime. SSD/local is much faster than network mounts; debug one platform first.

**Rename becomes a new game**  
Unique key wrongly used path/filename—implementation bug; merge on sha1+size.

**Chinese title overwritten by scrape**  
Title may be unlocked; after lock, only manual edits apply.

**Container up but no ROMs**  
Confirm compose mounts host ROMs at **`/app/roms`** and `paths.roms` is `/app/roms`.

**Database locked**  
Ensure WAL + busy_timeout; avoid multiple writers on one DB (single writer worker).

**Only data + config mounted, no roms**  
Point ROM path at `/app/data/roms` and place files there; still prefer a dedicated `/app/roms` volume.

---

## 8. Versioning & release (placeholder)

- Image tags: `retrove:<version>`  
- Before release, run the checklist “v1 acceptance” path  
- Changelog must note migrations that need manual steps (breaking migrations required)

---

## 9. Keeping docs fresh

- Sync this file when adding env vars, ports, or mounts.  
- When checklist phases finish, replace “TBD commands” here with real `make`/`docker` commands.  
- Design-level changes: update [development.md](./development.md) first, then code and this guide.
