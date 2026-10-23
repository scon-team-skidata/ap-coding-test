BINARY := bin/shelly-simulator
IMAGE  := shelly-simulator

.PHONY: test-race all build run test vet fmt fmt-check lint docker-build docker-run compose-up compose-down clean

all: build

build:
	go build -o $(BINARY) .

run:
	go run .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

fmt-check:
	@files="$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))"; \
	test -z "$$files" || (echo "unformatted files:"; echo "$$files"; exit 1)

lint: fmt-check vet
	golangci-lint run

docker-build:
	docker build -t $(IMAGE) .

docker-run:
	docker compose -f ../docker-compose.yml up --build simulator

compose-up:
	$(MAKE) -C .. up

compose-down:
	$(MAKE) -C .. down

clean:
	rm -rf bin

test-race:
	go test -race ./...
