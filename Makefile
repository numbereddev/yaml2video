# Makefile for yaml2video
# Copyright (c) 2026 Ondics GmbH

DOCKER_IMAGE ?= yaml2video:local
DC ?= docker compose
ARGS ?= --help
MOUNT_DIR ?= ./examples

export DOCKER_IMAGE MOUNT_DIR FILE

.PHONY: help up down down-volumes build bash test

.DEFAULT_GOAL := help

help: ## Show this help
	@echo "# yaml2video make helpers"
	@echo "# Commands: make ..."
	@sed -ne '/@sed/!s/## //p' $(MAKEFILE_LIST)

up: ## Start the API server (MOUNT_DIR=./examples mounted at /data)
	@$(DC) up -d

down: ## Take down the yaml2video API server
	@$(DC) down

down-volumes: ## Take down the yaml2video API server and the volumes
	@$(DC) down --volumes

# Pass arguments with: make run ARGS="--help"
run: ## Run yaml2video with ARGS in the container
	@$(DC) run --rm --volume "$(CURDIR):/data" yaml2video $(ARGS)

check: ## Checks a yaml schema
	@$(DC) run --rm --volume "$(CURDIR):/data" yaml2video -n $(FILE)

logs: ## Show logs (stop with ctrl-c)
	$(DC) logs -f -t --tail=100

ps: ## What's up?
	$(DC) ps

config: ## Show composed docker-compose config
	$(DC) config

build: ## Build the yaml2video Docker image
	$(DC) build yaml2video

bash: ## Open Bash with the current directory mounted at /data
	$(DC) run --rm --entrypoint /bin/bash --volume "$(CURDIR):/data" yaml2video

test: ## Run all Go tests
	@$(DC) run --rm --entrypoint go --workdir /src --volume "$(CURDIR):/src" yaml2video test ./...
