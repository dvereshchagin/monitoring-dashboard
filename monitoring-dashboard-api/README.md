# Monitoring Dashboard

Real-time system monitoring dashboard built with **Go**, **WebSocket**, **HTMX**, and **Templ** using **DDD** and **Clean Architecture** principles.

## Features

- ✅ **Real-time metrics** - CPU, Memory, Disk, Network usage updated every 2 seconds via WebSocket
- ✅ **Historical data** - Store and visualize metrics history with Chart.js
- ✅ **Clean Architecture** - Domain-driven design with clear separation of concerns
- ✅ **Type-safe templates** - Using Templ for compile-time checked HTML templates
- ✅ **PostgreSQL** - Persistent storage with efficient indexing
- ✅ **Alerts** - Automatic warnings for critical system states

## Architecture

### Clean Architecture Layers

```
┌─────────────────────────────────────────────────────────┐
│  Interfaces (HTTP, WebSocket, Views)                    │
│  ↓ depends on                                            │
│  Infrastructure (PostgreSQL, gopsutil, WebSocket Hub)   │
│  ↓ depends on                                            │
│  Application (Use Cases, DTOs, Ports)                   │
│  ↓ depends on                                            │
│  Domain (Entities, Value Objects, Services)             │
└─────────────────────────────────────────────────────────┘
```

**Key Principles:**
- Dependencies point INWARD (Dependency Inversion)
- Domain layer has NO external dependencies
- Infrastructure implements Domain interfaces
- Use Cases coordinate domain objects

### Project Structure

```
monitoring-dashboard/
├── cmd/server/main.go              # Application entry point with DI
├── internal/
│   ├── domain/                     # Domain Layer (Business Logic)
│   │   ├── entity/                 # Entities (Metric)
│   │   ├── valueobject/           # Value Objects (MetricType, etc.)
│   │   ├── service/               # Domain Services
│   │   └── repository/            # Repository interfaces
│   ├── application/                # Application Layer (Use Cases)
│   │   ├── usecase/               # Use Cases
│   │   ├── dto/                   # Data Transfer Objects
│   │   └── port/                  # Ports (interfaces for infrastructure)
│   ├── infrastructure/             # Infrastructure Layer (Adapters)
│   │   ├── persistence/postgres/  # PostgreSQL repository
│   │   ├── collector/             # System metrics collectors (gopsutil)
│   │   └── notification/websocket/# WebSocket hub
│   └── interfaces/                 # Interfaces Layer (Controllers)
│       ├── http/handler/          # HTTP handlers
│       └── view/                  # Templ templates
├── pkg/                            # Shared utilities
│   ├── config/                    # Configuration
│   └── logger/                    # Logger
└── web/static/                     # Frontend assets (CSS, JS)
```

## Prerequisites

- **Go 1.24+**
- **PostgreSQL 14+**
- **templ CLI** (will be installed via `go install`)

## Installation

### 1. Clone the repository

```bash
cd /Users/davereschagin/GolandProjects/monitoring-dashboard
```

### 2. Install dependencies

```bash
go mod download
go install github.com/a-h/templ/cmd/templ@latest
```

### 3. Setup PostgreSQL database

Create database:

```bash
createdb monitoring
# or
psql -U postgres -c "CREATE DATABASE monitoring;"
```

### 4. Configure environment variables

Copy `.env.example` to `.env` and adjust if needed:

```bash
cp .env.example .env
```

Edit `.env`:

```bash
SERVER_PORT=8080

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=monitoring

METRICS_COLLECTION_INTERVAL=2s
METRICS_RETENTION_DAYS=7
LOG_LEVEL=info

ALLOWED_ORIGINS=http://localhost:8080,http://127.0.0.1:8080
AUTH_ENABLED=true
AUTH_BEARER_TOKEN=your-strong-token

S3_ENABLED=true
S3_BUCKET=your-bucket
S3_REGION=ru-central1
S3_ENDPOINT=https://storage.yandexcloud.net
S3_ACCESS_KEY_ID=your-access-key
S3_SECRET_ACCESS_KEY=your-secret-key
S3_USE_PATH_STYLE=true
S3_KEY_PREFIX=dashboards
S3_URL_MODE=presigned
S3_PRESIGNED_TTL=5m

SCREENSHOT_MAX_PAYLOAD_MB=20
SCREENSHOT_MAX_ARTIFACT_MB=5
SCREENSHOT_RATE_LIMIT_PER_MINUTE=30
```

### 5. Run database migrations

```bash
make migrate
```

Or manually:

```bash
psql -U postgres -d monitoring -f internal/infrastructure/persistence/postgres/migrations/001_init.sql
psql -U postgres -d monitoring -f internal/infrastructure/persistence/postgres/migrations/002_indexes.sql
```

## Usage

### Build and run

```bash
make run
```

Or build binary:

```bash
make build
./bin/monitoring-dashboard
```

### Access the dashboard

Open your browser and navigate to:

```
http://localhost:8080
```

You should see:
- **4 metric cards** showing current CPU, Memory, Disk, and Network usage
- **Real-time updates** every 2 seconds via WebSocket
- **Historical charts** for CPU and Memory (last 1 hour)

## Development

### Generate Templ templates

After modifying `.templ` files:

```bash
make generate-templ
```

### Run tests

```bash
make test
```

### Lint and local CI checks

```bash
# install/update linter
make lint-install

# run linter
make lint

# run local CI checks (templ + gofmt + vet + test + build)
make ci-local
```

### Update tooling versions (Go + golangci-lint)

If you see an error like:
`the Go language version used to build golangci-lint is lower than the targeted Go version`,
update your local tooling:

```bash
go version
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint version
```

The CI pipeline uses the Go version from `go.mod`, so local tools should be updated to match.

### Clean build artifacts

```bash
make clean
```

## API Endpoints

### HTTP Endpoints

- `GET /` - Dashboard page
- `GET /api/v1/metrics/history?type={type}&duration={duration}` - Historical metrics
  - Example: `/api/v1/metrics/history?type=cpu&duration=1h`
- `POST /api/v1/screenshots/dashboard` - Save CPU/RAM/Disk/Network cards + CPU/Memory charts to S3-compatible storage
  - Always requires `Authorization: Bearer <token>` (`AUTH_BEARER_TOKEN` must be set)

### WebSocket Endpoint

- `WS /ws` - Real-time metrics stream

**Message format:**

```json
{
  "type": "snapshot",
  "data": {
    "timestamp": "2026-01-15T10:00:00Z",
    "cpu": {
      "id": "uuid",
      "type": "cpu",
      "value": 45.2,
      "unit": "%",
      "is_critical": false,
      "is_warning": false
    },
    "memory": { ... },
    "disk": { ... },
    "network": { ... },
    "summary": {
      "total_metrics": 4,
      "critical_count": 0,
      "warning_count": 0,
      "overall_status": "healthy"
    }
  }
}
```

## Configuration

### Metrics Collection

Metrics are collected every **2 seconds** by default. Adjust in `.env`:

```bash
METRICS_COLLECTION_INTERVAL=2s
```

### Data Retention

Metrics older than **7 days** are kept by default:

```bash
METRICS_RETENTION_DAYS=7
```

### Thresholds

**Warning thresholds** (defined in domain layer):
- CPU, Memory, Disk: > 75%
- Network: > 50 MB/s

**Critical thresholds**:
- CPU, Memory, Disk: > 90%
- Network: > 100 MB/s

## Technology Stack

### Backend

- **Go 1.24** - Main language
- **PostgreSQL** - Persistent storage
- **gorilla/websocket** - WebSocket connections
- **gopsutil** - System metrics collection
- **a-h/templ** - Type-safe HTML templates

### Frontend

- **HTMX** - Dynamic HTML updates
- **Chart.js** - Real-time charts
- **Vanilla JavaScript** - WebSocket client
- **CSS Grid** - Responsive layout

## Performance

- **Batch inserts** - Metrics inserted in batches every 2 seconds
- **Indexed queries** - Efficient time-range queries with PostgreSQL indexes
- **Buffered channels** - WebSocket hub uses buffered channels (256) for backpressure
- **Connection pooling** - PostgreSQL connection pool (25 max open, 5 max idle)

## Troubleshooting

### WebSocket connection fails

Check browser console for errors:

```javascript
// Expected in DevTools → Network → WS:
ws://localhost:8080/ws [connected]
```

### Metrics not updating

1. Check if collector is running:
   ```bash
   # Should see in logs:
   # "Metrics collector started"
   # "WebSocket hub started"
   ```

2. Verify database connection:
   ```bash
   psql -U postgres -d monitoring -c "SELECT COUNT(*) FROM metrics;"
   ```

### Database connection error

Verify PostgreSQL is running:

```bash
pg_isready -h localhost -p 5432
```

Check credentials in `.env` file.

## Future Enhancements

- [ ] Multiple host monitoring
- [ ] Alert notifications (email, Slack)
- [ ] Metrics aggregation (minute/hour rollups)
- [ ] Export to CSV/JSON
- [ ] User authentication
- [ ] Docker containerization
- [ ] Process monitoring

## License

MIT

## Author

Built with Clean Architecture and DDD principles for educational purposes.
