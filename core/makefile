.PHONY: build run test clean

build:
	go build -o bin/kora ./cmd/kora

run:
	go run ./cmd/kora

test:
	go test ./...

clean:
	rm -rf bin/

deps:
	go mod tidy
