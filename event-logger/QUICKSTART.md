# Event Logger - Quick Start Guide

## Local Development

### 1. Prerequisites

```bash
# Install dependencies
- Go 1.23+
- PostgreSQL 16+
- NATS Server with JetStream
- Docker (optional)
```

### 2. Start NATS Server

```bash
# Using Docker
docker run -d --name nats-jetstream \
  -p 4222:4222 \
  -p 8222:8222 \
  nats:2.10-alpine \
  -js

# Or download and run locally
nats-server -js
```

### 3. Setup Database

```bash
# Create database
createdb monitoring_dashboard

# Run migration
psql -d monitoring_dashboard -f internal/infrastructure/persistence/postgres/migrations/001_create_events.sql
```

### 4. Configure Environment

```bash
# Copy example env file
cp .env.example .env

# Edit .env with your settings
vi .env
```

### 5. Run Event Logger

```bash
# Install dependencies
go mod tidy

# Run the service
go run ./cmd/event-logger

# Or using Makefile
make run
```

The service will start on `http://localhost:8083`

### 6. Test Manually

```bash
# Test health check
curl http://localhost:8083/healthz

# Publish a test event to NATS
nats pub events.metrics.collected '{
  "event_type": "metric.collected",
  "aggregate_id": "test-metrics-1",
  "aggregate_type": "metrics",
  "payload": {
    "metrics_count": 5,
    "cpu_usage": 45.2,
    "memory_usage": 78.3
  },
  "version": 1
}'

# Query events
curl http://localhost:8083/api/v1/events?event_type=metric.collected
```

## Docker Development

### Build Image

```bash
make docker-build
```

### Run Container

```bash
make docker-run
```

Or manually:

```bash
docker run -d \
  -p 8083:8083 \
  -e DB_HOST=host.docker.internal \
  -e DB_PORT=5432 \
  -e DB_USER=postgres \
  -e DB_PASSWORD=postgres \
  -e DB_NAME=monitoring_dashboard \
  -e NATS_URL=nats://host.docker.internal:4222 \
  event-logger:latest
```

## Testing

### Run Unit Tests

```bash
make test
```

### Run with Coverage

```bash
make test-coverage
```

### Test Event Flow

```bash
# Terminal 1: Watch logs
make run

# Terminal 2: Publish events
nats pub events.metrics.collected '{...}'

# Terminal 3: Query events
curl http://localhost:8083/api/v1/events
```

## API Examples

### Get all events (paginated)

```bash
curl "http://localhost:8083/api/v1/events?offset=0&limit=100"
```

### Get events by type with time filter

```bash
curl "http://localhost:8083/api/v1/events?event_type=metric.collected&from=2024-01-01T00:00:00Z&to=2024-12-31T23:59:59Z&limit=50"
```

### Get events for specific aggregate (replay)

```bash
curl "http://localhost:8083/api/v1/events?aggregate_id=metrics-batch-12345&aggregate_type=metrics"
```

### Check health

```bash
# Liveness
curl http://localhost:8083/healthz

# Readiness (checks DB and NATS)
curl http://localhost:8083/readyz
```

## Troubleshooting

### NATS Connection Failed

```bash
# Check NATS is running
curl http://localhost:8222/varz

# Check NATS URL in config
echo $NATS_URL
```

### Database Connection Failed

```bash
# Test database connection
psql -h localhost -U postgres -d monitoring_dashboard -c "SELECT 1"

# Check environment variables
env | grep DB_
```

### No Events Being Logged

```bash
# Check NATS streams exist
nats stream ls

# Check consumer is connected
nats consumer ls METRICS_EVENTS

# Check Event Logger logs
# Look for "Subscribed to NATS subject" messages
```

## Development Commands

```bash
# Format code
make fmt

# Lint code
make lint

# Tidy dependencies
make tidy

# Build binary
make build

# Clean artifacts
make clean
```

## Architecture

```
HTTP Request
     ↓
┌─────────────────┐
│   HTTP Router   │
└────────┬────────┘
         ↓
┌─────────────────┐
│  Query Handler  │
└────────┬────────┘
         ↓
┌─────────────────┐
│ QueryEventsUC   │
└────────┬────────┘
         ↓
┌─────────────────┐
│ Event Repository│
└────────┬────────┘
         ↓
┌─────────────────┐
│   PostgreSQL    │
└─────────────────┘


NATS Message
     ↓
┌─────────────────┐
│ Event Consumer  │
└────────┬────────┘
         ↓
┌─────────────────┐
│  LogEventUC     │
└────────┬────────┘
         ↓
┌─────────────────┐
│ Event Repository│
└────────┬────────┘
         ↓
┌─────────────────┐
│   PostgreSQL    │
└─────────────────┘
```

## Project Structure

```
event-logger/
├── cmd/event-logger/         # Application entry point
├── internal/
│   ├── domain/               # Core business logic (no dependencies)
│   ├── application/          # Use cases (orchestration)
│   ├── infrastructure/       # External adapters (DB, NATS)
│   └── interfaces/           # HTTP handlers, routers
├── pkg/                      # Shared packages
├── Dockerfile               # Container image
├── Makefile                 # Build automation
└── README.md                # Full documentation
```

## Next Steps

1. Read [README.md](README.md) for detailed documentation
2. Explore the code starting from `cmd/event-logger/main.go`
3. Review Clean Architecture layers in `internal/`
4. Check out the migration in `internal/infrastructure/persistence/postgres/migrations/`
5. Test locally before deploying to Kubernetes

## Support

For issues or questions:
- Check [CLAUDE.md](../monitoring-dashboard-api/CLAUDE.md) for architecture principles
- Consult NATS documentation: https://docs.nats.io/
- Review git history for implementation details
