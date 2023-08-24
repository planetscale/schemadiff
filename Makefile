all: build test

CGO_ENABLED=0

build:
	go build -trimpath -o bin/schemadiff ./cmd/schemadiff/main.go

lint:
	golangci-lint run ./... --timeout 5m

test:
	go test ./...
