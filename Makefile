all: build test

export CGO_ENABLED=0

build:
	go build -trimpath -ldflags="-s -w" -o bin/schemadiff ./cmd/schemadiff/main.go

lint:
	golangci-lint run ./... --timeout 5m

test:
	go test ./...
