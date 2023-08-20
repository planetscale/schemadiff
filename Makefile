all: build test

build:
	go build -o bin/schemadiff ./cmd/schemadiff/main.go

lint:
	golangci-lint run ./...

test:
	go test ./...
