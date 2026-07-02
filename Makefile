.PHONY: build run test fmt tidy

build:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/proxy ./cmd/proxy

run:
	go run ./cmd/proxy

test:
	go test ./...

fmt:
	gofmt -l -w .
	go vet ./...

tidy:
	go mod tidy
