.PHONY: build test test-race vet fmt check run compose-up compose-down

build:
	go build -o bin/shelly-agent .

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w $$(find . -name '*.go' -not -path './vendor/*')

check: test vet

run:
	go run .

compose-up:
	$(MAKE) -C .. up

compose-down:
	$(MAKE) -C .. down
