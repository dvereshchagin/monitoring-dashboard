.PHONY: help build run migrate generate-templ test lint lint-install goose-install ci-local clean dev

help:
	@echo "Available commands:"
	@echo "  make build          - Build the application"
	@echo "  make run            - Run the application"
	@echo "  make migrate        - Run database migrations"
	@echo "  make generate-templ - Generate templ templates"
	@echo "  make test           - Run tests"
	@echo "  make lint-install   - Install/update golangci-lint"
	@echo "  make goose-install  - Install/update goose"
	@echo "  make lint           - Run golangci-lint"
	@echo "  make ci-local       - Run local CI checks"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make dev            - Run in development mode"

build: generate-templ
	go build -o bin/monitoring-dashboard cmd/server/main.go

run: generate-templ
	go run cmd/server/main.go

migrate:
	@echo "Running migrations..."
	@goose -dir internal/infrastructure/persistence/postgres/migrations postgres "host=localhost port=5432 user=postgres password=postgres dbname=monitoring sslmode=disable" up
	@echo "Migrations completed"

generate-templ:
	@echo "Generating templ templates..."
	@templ generate
	@echo "Templ generation completed"

test:
	go test -v ./...

lint-install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

goose-install:
	go install github.com/pressly/goose/v3/cmd/goose@latest

lint:
	golangci-lint run --timeout=5m

ci-local: generate-templ
	@git diff --exit-code
	@UNFORMATTED="$$(gofmt -l . | grep -v '^vendor/' || true)"; \
	if [ -n "$$UNFORMATTED" ]; then \
		echo "The following files need gofmt:"; \
		echo "$$UNFORMATTED"; \
		exit 1; \
	fi
	go vet ./...
	go test -v ./...
	go build -o bin/monitoring-dashboard cmd/server/main.go
	@echo "Local CI checks completed"

clean:
	rm -rf bin/
	find internal/interfaces/view -name "*_templ.go" -delete
	@echo "Cleaned build artifacts"

dev: generate-templ
	@echo "Running in development mode..."
	@go run cmd/server/main.go
