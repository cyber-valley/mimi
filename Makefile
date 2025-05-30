format:
	go fmt ./...

lint: format
	go vet ./...
	staticcheck ./...

pre-commit: lint

run: lint
	go run main.go

generate-requirements:
	uv export --no-hashes --format requirements-txt > requirements.txt
