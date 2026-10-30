# Retrove

[中文说明](README.zh-CN.md)

Self-hosted ROM library manager for retro gaming. Scan local folders, identify games by content hash, scrape metadata and media, manage Chinese/English titles in a web UI, and export to frontends such as Pegasus.

Inspired by tools like RomM, with Chinese titles and China-oriented frontend export as first-class features.

> Retrove does **not** include or distribute ROMs. You must supply files you are legally allowed to use.

## Features

- Multi-platform ROM libraries (FC / SFC / MD / FBNeo / MAME, and more)
- Stable identity via content hash (rename-safe): console ROMs hash inner payload; arcade hashes the zip
- One file → one game entry (regions, hacks, and translations stay separate)
- Pegasus import is authoritative for Chinese titles, descriptions, developers, box art, logos, and preview videos, and can be re-imported
- Translation writes English titles. Scrape (IGDB → ScreenScraper → SteamGridDB) only fills empty fields and missing media
- Web console: browse, edit one game, and write the Pegasus ROM name, metadata, and media folder back; multi-theme UI (zh/en)
- Export toward Pegasus and other frontend layouts (see docs)
- Docker / Compose friendly; single Go binary + Vue admin

## Stack

| Layer    | Choice                                      |
|----------|---------------------------------------------|
| Backend  | Go + SQLite (WAL)                           |
| Frontend | Vue 3 + TypeScript + Vite + Naive UI        |
| Deploy   | Docker / Docker Compose                     |

## Quick start

### Requirements

- Go (see `go.mod`)
- Node.js 20+ (frontend)
- Optional: Docker + Docker Compose

### Local development

```bash
# Create runtime dirs and copy config if missing
make config

# Edit credentials (never commit this file)
cp app/config/config.example.yaml app/config/config.yaml
# fill ScreenScraper / IGDB / SteamGridDB / LLM keys as needed

# Terminal 1 — API
make dev-backend

# Terminal 2 — admin UI
make dev-frontend
```

- API health: http://127.0.0.1:5080/api/v1/health  
- Admin UI: http://127.0.0.1:5180  

Put ROMs under `app/roms/<Platform>/` (folder names map via `app/config/platforms.yaml`).

### Docker

```bash
make config
# edit app/config/config.yaml, then:
make docker-up
```

Compose mounts `./app/config`, `./app/data`, and `./app/roms` into the container. Default port: **5080**.

## Configuration

| Path | Purpose |
|------|---------|
| `app/config/config.example.yaml` | Committed template |
| `app/config/config.yaml` | Local secrets (gitignored) |
| `app/config/platforms.yaml` | Directory name ↔ platform / ScreenScraper system id / IGDB platform id |
| `app/config/dictionaries/` | Chinese dictionary import/export |
| `app/config/retrove.db` | SQLite database (gitignored) |
| `app/data/` | Scraped media, cache, export output |
| `app/roms/` | Your ROM tree |

Scraping credentials live only in `config.yaml` (or env overrides). Leave a provider empty to disable it; the rest of the app still works.

## Documentation

- [Development guide](docs/development.md)
- [Development checklist](docs/checklist.md)
- [Deployment guide](docs/deployment.md)

## Repository layout

```text
retrove/
├── cmd/retrove/       # Go entrypoint
├── internal/          # Backend packages
├── web/               # Vue admin
├── app/               # Runtime only (config / data / roms)
├── docs/              # Design & ops (EN + 中文)
├── docker-compose.yml
├── Dockerfile
└── Makefile
```

## License

No license file is published yet. If you fork or redistribute, clarify licensing with the maintainers before shipping binaries that embed third-party API credentials.
