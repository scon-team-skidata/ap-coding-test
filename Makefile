PROJECTS := shelly-agent shelly-simulator shelly-monitoring-ui

.PHONY: up up-secure down logs ps build test test-race vet check

up:
	docker compose up --build -d

up-secure:
	docker compose -f docker-compose.yml -f docker-compose.secure.yml up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

ps:
	docker compose ps

build test test-race vet:
	@set -e; for project in $(PROJECTS); do $(MAKE) -C $$project $@; done

check: test vet
