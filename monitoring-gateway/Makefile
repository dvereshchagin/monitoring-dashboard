BINARY_NAME=monitoring-gateway

.PHONY: build test run lint lint-install

build:
	go build -o bin/$(BINARY_NAME) ./cmd/monitoring-gateway

test:
	go test -v ./...

run:
	go run ./cmd/monitoring-gateway

lint:
	@if [ -f ../.golangci.yml ]; then \
		golangci-lint run --config=../.golangci.yml --timeout=5m; \
	else \
		golangci-lint run --timeout=5m; \
	fi

lint-install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
