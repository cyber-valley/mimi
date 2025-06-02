-include .env

format:
	go fmt ./...

lint: format
	go vet ./...
	staticcheck ./...

pre-commit: lint

run-telegram-bot: lint
	go run ./cmd/bot/main.go

sqlc-generate:
	podman run --rm -v $(PWD):/src -w /src docker.io/sqlc/sqlc generate

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
