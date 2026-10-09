DOCKER_IMAGE ?= yaml2video:local
COMPOSE ?= docker compose
ARGS ?= -t examples/simple/template-simple.yaml -o examples/simple/output.mp4 examples/simple/video-simple.yaml
DOCKER_ARGS ?= $(ARGS)

export DOCKER_IMAGE

.PHONY: run build install test docker-build docker-run

run:
	@$(COMPOSE) run --rm --build development run ./src/cmd/yaml2video $(ARGS)

build:
	@$(COMPOSE) run --rm --build development build ./src/cmd/yaml2video

install:
	@$(COMPOSE) run --rm --build development install ./src/cmd/yaml2video

test:
	@$(COMPOSE) run --rm --build development test ./...

docker-build:
	$(COMPOSE) build yaml2video

docker-run:
	$(COMPOSE) run --rm --build yaml2video $(DOCKER_ARGS)
