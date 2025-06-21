.PHONY: format lint
include .env.example
-include .env
db-container = mimi-db

install:
	wget -O $(HOME)/.local/bin/sleek \
		https://github.com/nrempel/sleek/releases/download/v0.5.0/sleek-linux-x86_64
	chmod +x $(HOME)/.local/bin/sleek
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

format:
	sleek ./sql/migrations/*
	sleek ./sql/queries/*
	golangci-lint fmt ./...

lint: format
	go vet ./...
	staticcheck ./...

pre-commit: lint

test:
	go test -v ./...

run-telegram-bot: lint
	air --build.cmd "go build -o tmp/bot cmd/bot/main.go" --build.bin "./bin/bot"

run-telegram-scraper: lint
	go run cmd/scraper/telegram.go

run-telegram-scraper-live-reload: lint
	air \
		--build.cmd "go build -o bin/scraper/telegram cmd/scraper/telegram.go" \
		--build.bin "./bin/scraper/telegram"

run-logseq-scraper: lint
	go run cmd/scraper/logseq/main.go

sqlc-generate:
	sqlc generate

migrate:
	go run ./cmd/migrate/main.go

podman-db:
	test -n "$(DB_USER)" || exit 1
	test -n "$(DB_PASSWORD)" || exit 1
	test -n "$(DB_NAME)" || exit 1
	test -n "$(DB_PORT)" || exit 1
	podman stop $(db-container) || exit 0
	podman run --rm -d \
		--name=$(db-container) \
		-e POSTGRES_PASSWORD="$(DB_PASSWORD)" \
		-e POSTGRES_USER="$(DB_USER)" \
		-e POSTGRES_DB="$(DB_NAME)" \
		-p "$(DB_PORT)":5432 \
		docker.io/pgvector/pgvector:pg17

psql:
	PGPASSWORD=$(DB_PASSWORD) psql -U $(DB_USER) -d $(DB_NAME) -p $(DB_PORT) -h localhost

build-image:
	test -n $(IMAGE_NAME) || exit 1
	podman build -t $(IMAGE_NAME) .
