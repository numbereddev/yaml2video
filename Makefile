DOCKER_IMAGE ?= yaml2video:local
DOCKER_ARGS ?= -t template.yaml -o output.mp4 video.yaml

.PHONY: run build install test docker-build docker-run

run:
	@go run ./src/cmd/yaml2video

build:
	@go build ./src/cmd/yaml2video

install:
	@go install ./src/cmd/yaml2video

test:
	@go test ./...

docker-build:
	docker build -f Dockerfile -t "$(DOCKER_IMAGE)" .

docker-run:
	docker run --rm -v "$(CURDIR):/data" "$(DOCKER_IMAGE)" $(DOCKER_ARGS)
