# Retrove — Development Guide

[中文](./开发文档.md)

> Self-hosted ROM library for Chinese retro-gaming users. Mirrors RomM’s “scan → identify → scrape → manage → export”, with Chinese titles and China-oriented frontends as first-class concerns.  
> This document covers product design. Tasks: [checklist.md](./checklist.md). Deploy: [deployment.md](./deployment.md).

---

## 1. Product positioning

Retrove helps users:

- Manage multi-system ROM trees (FC / SFC / MD / FBNeo / MAME …)
- Identify files stably by content hash (renames keep metadata)
- Treat Pegasus import as the source for Chinese titles, descriptions, developers, box art, logos, and preview videos; translation supplies English names; scrape only fills gaps
- Edit one game in the web console and write the Pegasus ROM name, metadata fields, and media folder back
- Export tables and media layouts for Pegasus, EmuELEC, iisu, Cocoon, and similar frontends

**Non-goals (v1):** emulator cores, online multiplayer, bundling/distributing ROMs, multi-tenant SaaS.

---

## 2. Design principles

1. **ROM files are the source of truth**  
   Binaries live only on disk; the DB stores indexes, metadata, and job state. After deleting the DB you can rescan + re-import dictionaries/Pegasus.

2. **Identity = content hash, not filename**  
   Store CRC32, MD5, and SHA1. Rename/move only updates path fields.  
   - Consoles (FC / SFC / MD, …): hash the **ROM inside the archive** (identity survives re-zip).  
   - Arcade (MAME / FBNeo): hash the **zip file itself**.

3. **One file → one game**  
   US / JP / Chinese patch / hack are separate rows. `SFC` vs `SFC hack` are separated by directory; never auto-merged.

4. **Chinese title lives in the database**  
   The UI edits SQLite (`games` / `aliases`). `/app/config/dictionaries/` is for import and backup export only.  
   Match order: already in DB → local Pegasus / dictionary → ScreenScraper for English + box art → unidentified.  
   Generic MT is not the default path; LLM is optional later.

5. **One internal model; frontends are adapters**  
   Exporters all read the same `games` / `rom_files` / `media` and write their own formats.

6. **No automatic on-disk renames by default**  
   Edit `title_zh` in the DB; physical rename needs an explicit switch. Arcade defaults to deny.

---

## 3. Tech stack (decided)

| Layer | Choice |
|-------|--------|
| Backend | Go |
| Frontend | Vue 3 + TypeScript + Vite + Naive UI |
| Database | SQLite (WAL) |
| Deploy | Docker / Docker Compose |
| Scraping (v1) | ScreenScraper + local dictionary |
| Later options | IGDB, LLM name cleanup, Redis job queue |

SQLite is enough for single-user; concurrent writes are serialized via one worker. Revisit PostgreSQL only if multi-user concurrency demands it.

---

## 4. Architecture

```text
┌─────────────────────────────────────────────────────────┐
│  Vue3 + Naive UI (admin)                                 │
└───────────────────────────┬─────────────────────────────┘
                            │ HTTP JSON API
┌───────────────────────────▼─────────────────────────────┐
│  Go API + in-process / side worker                       │
│  scan │ match │ scrape │ dictionary │ export │ jobs     │
└───────┬─────────────────┬─────────────────┬─────────────┘
        │                 │                 │
   ┌────▼────┐   ┌──────▼──────┐  ┌──────▼──────┐
   │ SQLite  │   │ /app/roms   │  │ /app/data   │
   │(app/data)│   │ ROM library │  │ media/      │
   └─────────┘   └─────────────┘  │ export/     │
                                  │ cache/      │
                                  └─────────────┘
                      ┌──────────────┐
                      │ /app/config  │ config, dictionaries, secrets
                      └──────────────┘
```

### 4.1 Backend modules

| Module | Responsibility |
|--------|----------------|
| `api` | HTTP routes, single-user session, DTOs |
| `scan` | Walk ROM tree, incremental hash, path-change detection |
| `match` | Identify via hash / dictionary / cleaned filename |
| `scrape` | ScreenScraper client, rate limits, persistence |
| `dictionary` | Dictionary file import/export; runtime reads DB |
| `import` | Import existing Pegasus `metadata.pegasus.txt` |
| `export` | Pegasus / EmuELEC / iisu / Cocoon adapters |
| `media` | Download, store, index under `/app/data/media` |
| `jobs` | Background job state (scan / scrape / export) |
| `store` | SQLite access and migrations |

### 4.2 Frontend modules (pages)

| Module | Function |
|--------|----------|
| Library overview | Stats, missing covers / unidentified / pending, shortcuts |
| Platform library | Platform tree + list/cards, filter & sort |
| Game detail | ZH/EN titles, blurb, ROM files, media; save writes back the Pegasus ROM name, metadata fields, and media folder |
| Scan & identify | Progress, add/remove/rename diff, bulk accept matches |
| Scrape center | ScreenScraper account, scrape by gap/platform, retry failures |
| Alias workbench | Browse local dictionary, bulk apply, review suggestions |
| Media library | Missing list, manual upload/replace |
| Export | Pick targets & platforms → `data/export` |
| Platform mapping | Folder name ↔ internal platform ↔ frontend system IDs |
| Settings | Paths, secrets, rename policy, logging |
| Jobs & logs | Job list, cancel, error detail |

v1 can ship first: overview, platform library, detail, scan, aliases, Pegasus export, settings, jobs.

---

## 5. Data model (conceptual)

### 5.1 Core entities

- **platforms**  
  Internal platform code, display name, directory rules, extensions, frontend mappings, auto-rename deny (Arcade).

- **games**  
  One game per primary file. Fields: `title_zh` / `title_en` / `title_ja`, description, developer, year, lock flags, scrape external IDs.

- **rom_files**  
  Relative path, filename, size, crc32, md5, sha1, `game_id`, missing.  
  **Business uniqueness:** content hash (inner ROM for consoles, zip for arcade) + size. Same hash on multiple paths = duplicates.

- **aliases**  
  In DB: platform + English name / nickname / historical filename → `game_id`. Dictionary imports write here.

- **media_assets**  
  Kind (box / title / screenshot / …), path relative to `data/media`, source.

- **scrape_cache**  
  Cache ScreenScraper raw/normalized payloads by hash or external ID.

- **jobs**  
  Type, progress, status, error, timestamps.

### 5.2 ROM uniqueness and renames

```text
Scan file
  → If size+mtime unchanged and path known: skip rehash
  → Console: extract primary ROM → crc32 / md5 / sha1
    Arcade: hash the zip itself
  → Lookup rom_files by that hash(+size)
       hit: update path / filename (rename/move)
       miss: insert rom_files + new game (one file, one game)
  → Rows not seen this pass: mark missing (do not delete metadata immediately)
```

Names hang off `games` and `aliases`, never as the file primary key.

### 5.3 Identification pipeline

1. Hash already linked to a `game` → keep Chinese title and scrape results  
2. Pegasus import (`metadata.pegasus.txt` + `media/{ROM stem}/`) overwrites Chinese title, description, developer, box, and logo  
3. Translation writes `title_en` from the Chinese title (skip when English already exists) so scrape can search  
4. Scrape only fills gaps Pegasus did not provide: IGDB → ScreenScraper → SteamGridDB; existing fields and media stay  
5. Still failing → `unidentified`; edit the game in the detail panel and write the Pegasus files back

---

## 6. Feature notes

### 6.1 Scan

- Recurse `/app/roms` (or configured root); assign platform by first-level (or agreed) subfolder.
- Extension allow-list; ignore hidden files and export artifacts.
- Report: added / renamed / missing / duplicate hash.

### 6.2 Local dictionary

- Runtime reads only DB `aliases` and `games.title_zh`.
- `/app/config/dictionaries/` for import and DB backup export.
- Entries at least: platform, English canonical name or hash, Chinese title, optional aliases.
- Pegasus import writes `games` and `aliases`, and overwrites the Chinese title, description, and developer.
- Alias workbench edits the DB. Scrape does not replace an existing Chinese title, description, developer, English title, or media file.

### 6.3 Scrapers (ScreenScraper + IGDB + SteamGridDB)

Batch scrape chains configured sources and only fills empty values. Provider order stays:

1. **IGDB** (primary, optional): Twitch Client ID/Secret. Translated English name + `igdb_id` from `platforms.yaml`. Fills empty English title, description, developer, year, box art, screenshots, and title art.
2. **ScreenScraper** (optional): developer/user credentials in `/app/config`. Prefer content hash, then filename; fills only what IGDB still missed.
3. **SteamGridDB** (optional): API Key. Fills remaining media only (portrait covers, title art, logos; hero as screenshot fallback).

Games without an English title are skipped. Preview video is not scraped. Results go to the database and `/app/data/media/{platform}/{game_id}/` only — not back into the Pegasus folders. Respect API quotas; failures go to jobs for retry.  
Do **not** rename ROM files from scrape results. Settings show provider status on the first level; credentials open in a nested dialog.

### 6.4 Import Pegasus metadata

Pegasus data is `metadata.pegasus.txt` plus `media/{ROM stem}/` in each platform folder. Re-import treats those files as authoritative and overwrites the matching library values.

- Align `file:` with scanned `rom_files`. Entries with no ROM row are skipped.
- `game:` → Chinese title, `description:` → description, `developer:` → developer.
- The media folder follows the ROM stem, not the `game:` title. `boxFront.*` → box, `logo.*` → logo, and `video.*` → preview video are copied to `data/media/{platform}/{game_id}/` with source `pegasus`. A missing file does not delete one already in the library.

Saving a game in the detail panel writes those files back:

- Chinese title: on host platforms, rename the ROM to `{new title}{original extension}`, update `rom_files`, and patch `game:` / `file:`. Rename `media/{old stem}/` with it, including the preview video. If the target ROM or media folder already exists, cancel the rename. Arcade `rename_policy: deny` leaves the ROM and media folder names unchanged and only updates `game:`.
- Description or developer: patch that one game's field. Do not rewrite the whole metadata file.
- Replacing box / logo / preview video from the detail panel overwrites `boxFront.*` / `logo.*` / `video.*`. Screenshots, title art, and the English title stay in the library. Scrape does not import or replace preview video.

### 6.5 Export

- Always read from DB; write to **`/app/data/export/<target>/`** (do not overwrite next to user ROMs unless allowed).
- Targets:
  - **Pegasus**: `metadata.pegasus.txt` + media layout (compatible with existing libs; implement first)
  - **EmuELEC / EmulationStation**: `gamelist.xml` + media conventions
  - **iisu / Cocoon**: each via its own adapter
- Every export produces a report (ok / skipped / failed).

### 6.6 Admin console summary

- Search (Chinese/English titles, filenames)
- Edit one game and write the Chinese title, description, developer, box, logo, and preview video back to the Pegasus files
- Manual cover/video upload (box / logo / video also written into the Pegasus media folder)
- Trigger scan / scrape / export
- Review duplicate ROMs, missing files, unidentified list

### 6.7 Auth

- Single user only. With no user yet, the UI shows registration only; after one user exists, registration is hidden and rejected.
- Register: username, email, password, confirm (≥ 8 chars, must match).
- Login: username + password. Session in HttpOnly cookie.
- `GET /api/v1/health` is unauthenticated.

### 6.8 Later options (placeholder, not v1)

- Extra scrapers such as IGDB  
- LLM structured rename suggestions (review queue; default no disk rename)  
- Multi-user  
- Redis distributed queue  

---

## 7. Directories and Docker mounts

Container working directory is `/app`; persistent dirs hang under `/app/`:

| Host (suggested) | In container | Contents |
|------------------|--------------|----------|
| `./app/config` | `/app/config` | `config.yaml`, dictionaries, ScreenScraper secrets, `retrove.db` |
| `./app/data` | `/app/data` | `media/`, `export/`, `cache/` |
| `./app/roms` | `/app/roms` | ROM library |

Notes:

- Source lives at repo root: `cmd/`, `internal/`, `web/`. `app/` is runtime-only (`config/`, `data/`, `roms/`).  
- Image ships the Go binary (admin SPA embedded) plus default `platforms.yaml` / `config.example.yaml`; Compose mounts only these three dirs—do not bind-mount the whole repo.  
- On first boot, the entrypoint seeds `platforms.yaml` / `config.yaml` into the mounted volume when missing.  
- In-container admin UI listens on `:5180` (published); API on `:5080` (internal; UI forwards `/api`).  
- Docs live in **`docs/`**. Local start via root **`Makefile`**.  
- Exports always go to **`/app/data/export`**. Media under **`/app/data/media`**.

---

## 8. API conventions

- REST JSON under `/api/v1`.  
- Resources: `/platforms`, `/games`, `/roms`, `/jobs`, `/scrape`, `/export`, `/dictionary`.  
- Long work creates a job and returns `job_id`; poll (or SSE later; poll is enough for v1).  
- Errors: `{ "code", "message", "details?" }`.  

Fill the route table / OpenAPI during implementation per [checklist.md](./checklist.md).

---

## 9. Platform mapping

Maintain `/app/config/platforms.yaml` (or equivalent):

- Directory name (e.g. `SFC`, `FBNEO STG`) → internal `platform_id`
- Extension list
- ScreenScraper system id
- IGDB platform id
- Per-export system/folder names
- `rename_policy`: `allow` | `deny` (Arcade = `deny`)

Existing `roms/*/metadata.pegasus.txt` trees are useful samples for platforms and Chinese titles.

---

## 10. Security and compliance

- Never log full API secrets.  
- Run the container with configurable `PUID/PGID` for NAS permissions.  
- Only manage user-owned ROMs; never ship ROM downloads in releases or docs.  
- Obey third-party ToS and rate limits (ScreenScraper, etc.).  

---

## 11. Doc map

| Document | Role |
|----------|------|
| This file | Overall design (“constitution” while coding) |
| [checklist.md](./checklist.md) | Phased tasks and done criteria |
| [deployment.md](./deployment.md) | Local debug and Docker deploy |

Change docs before code when the design shifts.
