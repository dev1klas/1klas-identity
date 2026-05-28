.PHONY: build run vet lint test test-unit test-integration tidy docker

build:
	mkdir -p bin
	go build -ldflags="-s -w" -o bin/server ./cmd/server

run:
	go run ./cmd/server

vet:
	go vet ./...

lint:
	golangci-lint run

test:
	go test -race -count=1 ./...

test-unit:
	go test -race -count=1 ./internal/...

test-integration:
	go test -race -count=1 -tags=integration ./test/...

tidy:
	go mod tidy

docker:
	docker build -t 1klas-identity:dev .

dev-up:
	docker compose up -d
