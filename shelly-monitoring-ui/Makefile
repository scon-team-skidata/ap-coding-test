.PHONY: test-race build run test vet compose-up compose-down

build:
	go build -o bin/shelly-monitoring-ui .

run:
	go run .

test:
	go test ./...

vet:
	go vet ./...

compose-up:
	$(MAKE) -C .. up

compose-down:
	$(MAKE) -C .. down

test-race:
	go test -race ./...
