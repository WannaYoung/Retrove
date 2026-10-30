.PHONY: dev-backend dev-frontend backend frontend build test lint fmt docker-up docker-down dirs config

BACKEND_ENV = \
	RETROVE_CONFIG=./app/config/config.yaml \
	RETROVE_LISTEN=:5080 \
	RETROVE_ROMS=./app/roms \
	RETROVE_DATA=./app/data \
	RETROVE_CONFIG_DIR=./app/config \
	RETROVE_SQLITE=./app/data/retrove.db \
	RETROVE_EXPORT=./app/data/export

dirs:
	mkdir -p app/config/dictionaries app/data/media app/data/export app/data/cache

config: dirs
	@test -f app/config/config.yaml || cp app/config/config.example.yaml app/config/config.yaml

dev-backend: config
	$(BACKEND_ENV) go run ./cmd/retrove

dev-frontend:
	cd web && npm install && npm run dev

# 兼容旧名字
backend: dev-backend
frontend: dev-frontend

build: config
	$(BACKEND_ENV) go build -o bin/retrove ./cmd/retrove
	cd web && npm install && npm run build

test:
	go test ./...

fmt:
	gofmt -w cmd internal

lint:
	@test -z "$$(gofmt -l cmd internal)"
	go vet ./...

docker-up: config
	docker compose up -d --build

docker-down:
	docker compose down
