DOCKER_IMAGE ?= yaml2video:local
COMPOSE ?= docker compose
ARGS ?= -t examples/simple/template-simple.yaml -o examples/simple/output.mp4 examples/simple/video-simple.yaml
DOCKER_ARGS ?= $(ARGS)

export DOCKER_IMAGE

.PHONY: help up run build install test docker-build docker-run

help: ## Show this help.
	@sed -ne '/@sed/!s/## //p' $(MAKEFILE_LIST)

up: ## Start the serving stack.
	 @$(COMPOSE) -f docker-compose.serve.yaml up --build -d

down: ## Take down the serving stack.
	@$(COMPOSE) -f docker-compose.serve.yaml down

down-volumes: ## Take down the serving stack.
	@$(COMPOSE) -f docker-compose.serve.yaml down --volumes

run: ## Run yaml2video with ARGS in the development container.
	 @$(COMPOSE) -f docker-compose.dev.yaml run --rm --build development run ./src/cmd/yaml2video $(ARGS)

build: ## Build yaml2video in the development container.
	 @$(COMPOSE) -f docker-compose.dev.yaml run --rm --build development build ./src/cmd/yaml2video

install: ## Install yaml2video in the development container.
	 @$(COMPOSE) -f docker-compose.dev.yaml run --rm --build development install ./src/cmd/yaml2video

test: ## Run all Go tests in the development container.
	 @$(COMPOSE) -f docker-compose.dev.yamlrun --rm --build development test ./...

docker-build: ## Build the yaml2video Docker image.
	 $(COMPOSE) build yaml2video

docker-run: ## Run the yaml2video Docker image with DOCKER_ARGS.
	 $(COMPOSE) run --rm --build yaml2video $(DOCKER_ARGS)
