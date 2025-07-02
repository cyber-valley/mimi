.PHONY: format vet
include .env.example
-include .env

run: vet
	go run cmd/app/main.go

install:
	wget -O $(HOME)/.local/bin/sleek https://github.com/nrempel/sleek/releases/download/v0.5.0/sleek-linux-x86_64 &
	wget -O $(HOME)/.local/bin/geni https://github.com/emilpriver/geni/releases/download/v1.1.6/geni-linux-amd64 &
	wait
	chmod +x $(HOME)/.local/bin/sleek
	chmod +x $(HOME)/.local/bin/geni
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

vet: format
	go vet ./...
	staticcheck ./...

format:
	go fmt ./...
	sleek ./sql/migrations/*
	sleek ./sql/queries/*

test: vet
	go test -v ./...

sqlc: format
	sqlc generate

migrate-up:
	geni up
	rm $(DATABASE_MIGRATIONS_FOLDER)/schema.sql # I found this file useless

migrate-down:
	geni down
	rm $(DATABASE_MIGRATIONS_FOLDER)/schema.sql # I found this file useless

db-container = mimi-db
dev-db:
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
