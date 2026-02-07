# Monitoring Dashboard - Development Rules

## Project Overview

Real-time system monitoring dashboard built with Go, WebSocket, HTMX, and Templ using DDD and Clean Architecture principles.

## Architecture Principles

### Clean Architecture Layers

```
Domain (Core) → Application (Use Cases) → Infrastructure (Adapters) → Interfaces (Controllers)
```

**Dependency Rule**: Dependencies point INWARD. Outer layers depend on inner layers, never the reverse.

### Layer Responsibilities

1. **Domain Layer** (`internal/domain/`)
   - Pure business logic
   - No external dependencies
   - Contains: Entities, Value Objects, Domain Services, Repository Interfaces
   - Rules:
     - NO imports from other layers
     - NO framework dependencies
     - Only standard library and domain code

2. **Application Layer** (`internal/application/`)
   - Use Cases (application-specific business rules)
   - DTOs for data transfer
   - Ports (interfaces for infrastructure)
   - Rules:
     - Can import from Domain layer only
     - Defines interfaces (ports) for Infrastructure
     - Use Cases orchestrate domain objects

3. **Infrastructure Layer** (`internal/infrastructure/`)
   - Implementations of domain/application interfaces
   - Database, collectors, WebSocket, external services
   - Rules:
     - Implements interfaces from Domain and Application
     - Can import Domain and Application
     - Contains adapters to external systems

4. **Interfaces Layer** (`internal/interfaces/`)
   - HTTP handlers, WebSocket handlers, views
   - Entry points to the application
   - Rules:
     - Depends on Application (Use Cases)
     - Converts HTTP/WS to Use Case calls
     - Handles presentation logic

## Domain-Driven Design Patterns

### Entities & Aggregates

- **Metric** is the main Aggregate Root
- Entities have identity and lifecycle
- Use factory methods: `NewMetric()`
- Encapsulate business logic in methods: `IsStale()`, `ExceedsThreshold()`

### Value Objects

- **Immutable**: MetricType, MetricValue, TimeRange
- No identity, compared by value
- Include validation in constructor: `NewMetricValue()`
- Example: `MetricValue{value: 45.2, unit: "%"}`

### Domain Services

- Logic that doesn't belong to a single entity
- MetricAggregator: aggregates multiple metrics
- Stateless services

### Repository Pattern

- Interface in Domain layer: `MetricRepository`
- Implementation in Infrastructure: `PostgresMetricRepository`
- Abstracts data access

### Use Cases

- One class per use case (SRP)
- `CollectMetricsUseCase`, `GetCurrentMetricsUseCase`
- Coordinates domain objects and infrastructure

## Coding Standards

### File Naming

- Snake_case for files: `metric_repository.go`
- PascalCase for types: `MetricRepository`
- camelCase for private fields: `metricType`

### Package Structure

```
domain/
  entity/metric.go          # Metric entity
  valueobject/metric_type.go # MetricType value object
  service/metric_aggregator.go
  repository/metric_repository.go # Interface
```

### Error Handling

```go
// Good: Wrap errors with context
if err := uc.repository.SaveBatch(ctx, metrics); err != nil {
    return fmt.Errorf("failed to save metrics: %w", err)
}

// Bad: Swallow errors or return without context
if err != nil {
    return err
}
```

### Context Usage

- Always pass `context.Context` as first parameter
- Use for cancellation, timeouts, and request-scoped values
- Don't store context in structs

```go
func (uc *CollectMetricsUseCase) Execute(ctx context.Context) error {
    // ...
}
```

### Dependency Injection

- Use constructor injection
- Dependencies as struct fields
- Initialize in `cmd/server/main.go`

```go
type CollectMetricsUseCase struct {
    collector  port.MetricsCollector
    repository domain.MetricRepository
    notifier   port.NotificationService
}

func NewCollectMetricsUseCase(
    collector port.MetricsCollector,
    repository domain.MetricRepository,
    notifier port.NotificationService,
) *CollectMetricsUseCase {
    return &CollectMetricsUseCase{
        collector:  collector,
        repository: repository,
        notifier:   notifier,
    }
}
```

## Testing Strategy

### Unit Tests

- Test each layer independently
- Mock dependencies using interfaces
- File naming: `metric_test.go`

```go
// Domain layer: No mocks needed (pure logic)
func TestMetric_IsStale(t *testing.T) {
    metric := createTestMetric()
    assert.True(t, metric.IsStale(1*time.Second))
}

// Application layer: Mock repository and ports
func TestCollectMetricsUseCase_Execute(t *testing.T) {
    mockRepo := &MockMetricRepository{}
    mockCollector := &MockMetricsCollector{}
    uc := NewCollectMetricsUseCase(mockCollector, mockRepo, nil)
    // ...
}
```

### Integration Tests

- Test with real database (test container)
- Test WebSocket connections
- Test end-to-end flows

## Database Guidelines

### Migrations

- Sequential numbering: `001_init.sql`, `002_indexes.sql`
- Always include `IF NOT EXISTS` and `IF EXISTS`
- Test migrations up AND down

### Repository Implementation

- Use prepared statements for security
- Handle NULL values properly
- Use transactions for batch operations

```go
func (r *PostgresMetricRepository) SaveBatch(ctx context.Context, metrics []*entity.Metric) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Batch insert logic...

    return tx.Commit()
}
```

## WebSocket Guidelines

### Hub Pattern

- Central hub manages all connections
- Use channels for registration, unregistration, broadcast
- Goroutine-safe with mutex for client map

### Client Connection

- Separate read and write goroutines
- Use buffered channels for backpressure
- Handle ping/pong for keep-alive
- Graceful disconnect handling

```go
type Client struct {
    hub  *Hub
    conn *websocket.Conn
    send chan *dto.MetricSnapshotDTO
}

func (c *Client) WritePump() {
    ticker := time.NewTicker(54 * time.Second)
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()

    for {
        select {
        case message, ok := <-c.send:
            if !ok {
                return
            }
            if err := c.conn.WriteJSON(message); err != nil {
                return
            }
        case <-ticker.C:
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}
```

## Frontend Guidelines

### Templ Templates

- Use type-safe templ syntax
- Pass DTOs, not domain entities
- Run `templ generate` before build

```templ
package view

import "github.com/dreschagin/monitoring-dashboard/internal/application/dto"

templ Dashboard(snapshot *dto.MetricSnapshotDTO) {
    // ...
}
```

### HTMX Usage

- Use for dynamic content updates
- Combine with WebSocket for real-time data
- Progressive enhancement approach

### WebSocket Client

- Implement reconnection logic with exponential backoff
- Update DOM efficiently
- Handle connection status UI

```javascript
class MetricsWebSocket {
    connect() {
        this.ws = new WebSocket(`ws://${location.host}/ws`);
        this.ws.onopen = () => this.onConnect();
        this.ws.onclose = () => this.reconnect();
        this.ws.onmessage = (e) => this.handleMessage(e);
    }

    reconnect() {
        this.reconnectDelay = Math.min(this.reconnectDelay * 2, 30000);
        setTimeout(() => this.connect(), this.reconnectDelay);
    }
}
```

## Performance Considerations

### Metrics Collection

- Batch insert metrics every 2 seconds
- Use goroutines for concurrent collection
- Implement graceful shutdown

### Database

- Use connection pooling
- Add indexes on frequently queried columns
- Consider partitioning for time-series data

### WebSocket

- Buffer channels to prevent blocking
- Limit max clients if needed
- Efficient JSON serialization

## Git Workflow

### Commit Messages

```
feat: add CPU metrics collector
fix: resolve WebSocket connection leak
refactor: extract metric validation to domain service
docs: update README with architecture diagram
```

### Branch Strategy

- `main`: stable code
- `feature/metric-collection`: feature branches
- `fix/ws-reconnect`: bug fix branches

## Configuration

### Environment Variables

```bash
# Required
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=monitoring

# Optional with defaults
SERVER_PORT=8080
METRICS_COLLECTION_INTERVAL=2s
LOG_LEVEL=info
```

### Config Struct

```go
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Metrics  MetricsConfig
}
```

## Security Checklist

- [ ] Validate all user inputs
- [ ] Use parameterized queries (prevent SQL injection)
- [ ] Implement rate limiting on WebSocket connections
- [ ] Configure CORS properly
- [ ] Use HTTPS in production
- [ ] Don't log sensitive data
- [ ] Set proper PostgreSQL connection limits

## Deployment Checklist

- [ ] Run `templ generate`
- [ ] Run database migrations
- [ ] Set environment variables
- [ ] Configure logging level
- [ ] Set up monitoring/alerting
- [ ] Test WebSocket connections
- [ ] Verify database connection pool

## Common Patterns

### Creating a New Use Case

1. Define interface in `application/port/` if needed
2. Create use case struct in `application/usecase/`
3. Inject dependencies via constructor
4. Implement `Execute(ctx context.Context) error`
5. Call from handler in `interfaces/http/handler/`

### Adding a New Metric Type

1. Add to `MetricType` enum in `domain/valueobject/metric_type.go`
2. Update validation in `Validate()` method
3. Create collector in `infrastructure/collector/`
4. Update UI to display new metric

### Creating a New HTTP Endpoint

1. Create handler method in `interfaces/http/handler/`
2. Inject required use case
3. Add route in `interfaces/http/router.go`
4. Add middleware if needed

## Resources

- **Clean Architecture**: Robert C. Martin
- **Domain-Driven Design**: Eric Evans
- **gorilla/websocket**: https://github.com/gorilla/websocket
- **gopsutil**: https://github.com/shirou/gopsutil
- **Templ**: https://templ.guide/

## Questions & Issues

For questions about:
- Architecture decisions → Review this file and plan file
- Implementation details → Check existing code patterns
- External libraries → Consult official documentation

## Development Commands

```bash
# Generate templ templates
make generate-templ

# Build application
make build

# Run application
make run

# Run migrations
make migrate

# Run tests
make test

# Clean generated files
make clean
```

## Remember

1. **Domain is King**: Business logic lives in domain layer
2. **Test Boundaries**: Test each layer independently
3. **Depend on Abstractions**: Use interfaces, not concrete types
4. **Single Responsibility**: One use case per struct
5. **Fail Fast**: Validate early, return errors with context
6. **Keep It Simple**: Don't over-engineer for future requirements
