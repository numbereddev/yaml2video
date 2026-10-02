.PHONY: run build install test

run:
	@go run .

build:
	@go build .

install:
	@go install .

test:
	@go test ./...
