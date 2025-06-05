-include .env
db-container = mimi-db

install:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

format:
	go fmt ./...

lint: format
	go vet ./...
	staticcheck ./...

pre-commit: lint

run-telegram-bot: lint
	air --build.cmd "go build -o bin/bot cmd/bot/main.go" --build.bin "./bin/bot"

run-telegram-scraper: lint
	go run cmd/scraper/telegram.go

run-telegram-scraper-live-reload: lint
	air \
		--build.cmd "go build -o bin/scraper/telegram cmd/scraper/telegram.go" \
		--build.bin "./bin/scraper/telegram"

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
