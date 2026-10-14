DOCKER_IMAGE ?= yaml2video:local
COMPOSE ?= docker compose

export DOCKER_IMAGE

.PHONY: help up down down-volumes build bash test

help: ## Show this help
	@sed -ne '/@sed/!s/## //p' $(MAKEFILE_LIST)

up: ## Start the yaml2video API server
	 @$(COMPOSE) up -d

down: ## Take down the yaml2video API server
	@$(COMPOSE) down

down-volumes: ## Take down the yaml2video API server and the volumes
	@$(COMPOSE) down --volumes

build: ## Build the yaml2video Docker image
	 $(COMPOSE) build yaml2video

bash: ## Open Bash with the current directory mounted at /data
		$(COMPOSE) run --rm --entrypoint /bin/bash --volume "$(CURDIR):/data" yaml2video

test: ## Run all Go tests
	 @$(COMPOSE) run --rm --entrypoint go --workdir /src --volume "$(CURDIR):/src" yaml2video test ./...
