# Repository Guidelines

## Skills Context
- `senior-golang-developer`: use senior-level Go engineering standards in all backend changes.
- `aws-certified-experts`: apply AWS Well-Architected and production-grade cloud engineering practices.
- Expectations for this skill:
  - design for readability, correctness, and maintainability first;
  - enforce strict error handling and clear boundaries between layers;
  - prefer explicit dependencies, deterministic behavior, and testable code;
  - challenge risky shortcuts in architecture, performance, and security.
  - design AWS infrastructure with security, reliability, performance efficiency, cost optimization, and operational excellence in mind;
  - prefer infrastructure as code (Terraform), least-privilege IAM, private networking, and managed services where appropriate;
  - require observability, backup/restore strategy, and rollback-safe deployment flows for production changes.

## Project Structure & Module Organization
- `monitoring-dashboard-api/`: Go backend, deployment scripts, Docker/Helm/Terraform-related assets.
  - `cmd/server/`: app entrypoint.
  - `internal/`: domain, application, infrastructure, HTTP handlers, templates.
  - `pkg/`: shared config/logger utilities.
  - `deploy/`, `scripts/`: CI/CD, migrations, runtime/deploy helpers.
- `monitoring-dashboard-web-ui/`: static frontend assets (`static/css`, `static/js`) served by the API.
- `.github/workflows/`: CI/CD pipelines.

## Build, Test, and Development Commands
Run commands from `monitoring-dashboard-api/` unless noted.
- `make build`: build backend binary to `bin/monitoring-dashboard`.
- `make test`: run Go unit tests (`go test -v ./...`).
- `make lint-install && make lint`: install and run `golangci-lint`.
- `make goose-install && make migrate`: install `goose` and apply DB migrations.
- `make ci-local`: run local CI checks (templ, gofmt, vet, tests, build).
- `go run cmd/server/main.go`: run API directly for development.

## Coding Style & Naming Conventions
- Use standard Go formatting (`gofmt`) and keep code lint-clean (`golangci-lint`).
- Follow Go naming: exported identifiers in `CamelCase`, internal/private in `camelCase`.
- Keep package names short, lowercase, and domain-oriented (`collector`, `handler`, `usecase`).
- Prefer small, focused files and explicit error handling (`if err != nil` with context).

## Testing Guidelines
- Use Go’s standard testing package (`*_test.go`, `TestXxx` naming).
- Place tests near the code they validate.
- Prioritize tests for `internal/application/usecase` and domain logic first.
- Before PR: run `make test` and `make ci-local`.

## Commit & Pull Request Guidelines
- Prefer clear, imperative commit messages; Conventional Commits are recommended (e.g., `feat(api): add screenshot auth`).
- Keep PRs focused and small.
- PR checklist:
  - describe what changed and why;
  - link related issue/task;
  - include test evidence (`make test`, lint output, or CI link);
  - include screenshots for UI-impacting changes.

## Security & Configuration Tips
- Never commit secrets (`DB_PASSWORD`, `AUTH_BEARER_TOKEN`, cloud keys).
- Use `.env` locally and GitHub Secrets in CI/CD.
- Validate migrations with `goose` before deploys.
