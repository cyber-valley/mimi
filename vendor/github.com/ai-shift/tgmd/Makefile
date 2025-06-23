run: format
	go run ./main.go

format:
	go fmt ./...

vet: format
	go vet ./...

test: vet
	go test
