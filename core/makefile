.PHONY: build run test clean

build:
	go build -o bin/sheldon ./cmd/sheldon

run:
	go run ./cmd/sheldon

test:
	go test ./...

clean:
	rm -rf bin/

deps:
	go mod tidy
