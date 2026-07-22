.PHONY: build run test tidy install

build:
	go build -o bin/mogotor ./cmd/mogotor

run: build
	./bin/mogotor

test:
	go test ./...

tidy:
	go mod tidy

install:
	./deploy/install.sh
