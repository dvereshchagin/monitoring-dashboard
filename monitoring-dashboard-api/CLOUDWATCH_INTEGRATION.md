# AWS CloudWatch Integration

This document describes the AWS CloudWatch integration for the Monitoring Dashboard, including CloudWatch Metrics and CloudWatch Logs.

## Overview

The monitoring dashboard now supports publishing system metrics to AWS CloudWatch Metrics and streaming application logs to AWS CloudWatch Logs. This enables:

- **CloudWatch Metrics**: Custom metrics for CPU, Memory, Disk, and Network usage
- **CloudWatch Logs**: Centralized structured logging with CloudWatch Logs Insights
- **CloudWatch Dashboards**: Create custom dashboards using published metrics
- **CloudWatch Alarms**: Set up alerts on metric thresholds (future enhancement)
- **Multi-region deployment**: Centralized monitoring across distributed systems

## Architecture

The integration follows Clean Architecture principles:

```
Application Layer (Ports)
    ↓
Infrastructure Layer (CloudWatch Publishers)
    ↓
AWS SDK v2 (cloudwatch, cloudwatchlogs)
```

**Key Components:**
- `internal/application/port/metrics_publisher.go` - Port interface for metrics publishing
- `internal/application/port/log_publisher.go` - Port interface for log publishing
- `internal/infrastructure/observability/cloudwatch/metrics_publisher.go` - CloudWatch Metrics implementation
- `internal/infrastructure/observability/cloudwatch/logs_publisher.go` - CloudWatch Logs implementation

## Configuration

### Environment Variables

```bash
# Enable/Disable Features
CLOUDWATCH_METRICS_ENABLED=true    # Enable CloudWatch Metrics publishing
CLOUDWATCH_LOGS_ENABLED=true       # Enable CloudWatch Logs streaming

# AWS Credentials
CLOUDWATCH_REGION=us-east-1
CLOUDWATCH_ACCESS_KEY_ID=your-access-key
CLOUDWATCH_SECRET_ACCESS_KEY=your-secret-key

# Optional: Override endpoint for LocalStack
# CLOUDWATCH_ENDPOINT=http://localhost:4566

# Metrics Configuration
CLOUDWATCH_METRICS_NAMESPACE=MonitoringDashboard/System
CLOUDWATCH_METRICS_BUFFER_SIZE=100
CLOUDWATCH_METRICS_FLUSH_INTERVAL=10s
CLOUDWATCH_METRICS_STORAGE_RESOLUTION=60  # 60=standard, 1=high-resolution

# Metrics Dimensions (comma-separated key=value pairs)
CLOUDWATCH_METRICS_DIMENSIONS=Environment=production,Host=server-01

# Logs Configuration
CLOUDWATCH_LOG_GROUP=/aws/monitoring-dashboard
CLOUDWATCH_LOG_STREAM=application
CLOUDWATCH_LOGS_BUFFER_SIZE=50
CLOUDWATCH_LOGS_FLUSH_INTERVAL=5s
```

### Configuration Defaults

| Setting | Default | Description |
|---------|---------|-------------|
| `CLOUDWATCH_METRICS_ENABLED` | `false` | Enable metrics publishing |
| `CLOUDWATCH_LOGS_ENABLED` | `false` | Enable logs streaming |
| `CLOUDWATCH_REGION` | `us-east-1` | AWS region |
| `CLOUDWATCH_METRICS_NAMESPACE` | `MonitoringDashboard/System` | Metrics namespace |
| `CLOUDWATCH_METRICS_BUFFER_SIZE` | `100` | Metrics buffer size |
| `CLOUDWATCH_METRICS_FLUSH_INTERVAL` | `10s` | Auto-flush interval for metrics |
| `CLOUDWATCH_METRICS_STORAGE_RESOLUTION` | `60` | Standard resolution (seconds) |
| `CLOUDWATCH_LOG_GROUP` | `/aws/monitoring-dashboard` | Log group name |
| `CLOUDWATCH_LOG_STREAM` | `application` | Log stream name |
| `CLOUDWATCH_LOGS_BUFFER_SIZE` | `50` | Logs buffer size |
| `CLOUDWATCH_LOGS_FLUSH_INTERVAL` | `5s` | Auto-flush interval for logs |

## Features

### CloudWatch Metrics

**Published Metrics:**
- `cpu_usage` - CPU utilization (%)
- `memory_usage` - Memory utilization (%)
- `disk_usage` - Disk utilization (%)
- `network_sent` - Network bytes sent (MB/s)

**Dimensions:**
Each metric includes the following dimensions for filtering:
- `MetricType` - Type of metric (cpu, memory, disk, network)
- `MetricName` - Specific metric name
- Custom dimensions from `CLOUDWATCH_METRICS_DIMENSIONS`

**Buffering & Batching:**
- Metrics are buffered to reduce API calls
- Auto-flush on buffer full (default: 100 metrics) or interval (default: 10s)
- Batch operations respect CloudWatch limit (1000 metrics/request)
- Graceful shutdown flushes all buffered metrics

**Retry Logic:**
- Exponential backoff with 3 retry attempts
- Handles transient AWS API errors
- Non-blocking: CloudWatch failures don't stop metric collection

### CloudWatch Logs

**Log Format:**
Structured JSON logs with the following fields:
```json
{
  "timestamp": "2026-02-08T12:00:00.000Z",
  "level": "INFO",
  "message": "Metrics collected successfully",
  "fields": {
    "count": 4,
    "duration_ms": 123
  }
}
```

**Log Levels:**
- `DEBUG` - Detailed debugging information
- `INFO` - Informational messages
- `WARN` - Warning messages
- `ERROR` - Error messages with stack traces

**Features:**
- Buffered publishing (default: 50 events, 5s flush)
- Chronological ordering (CloudWatch requirement)
- Auto-create log group/stream on first publish
- Sequence token management for ordering
- Truncation for oversized events (max 256 KB)

## Usage

### Basic Setup

1. **Enable CloudWatch integration:**
   ```bash
   export CLOUDWATCH_METRICS_ENABLED=true
   export CLOUDWATCH_LOGS_ENABLED=true
   export CLOUDWATCH_REGION=us-east-1
   export CLOUDWATCH_ACCESS_KEY_ID=your-key
   export CLOUDWATCH_SECRET_ACCESS_KEY=your-secret
   ```

2. **Start the application:**
   ```bash
   make run
   ```

3. **Verify in AWS Console:**
   - Navigate to CloudWatch → Metrics → Custom Namespaces → `MonitoringDashboard/System`
   - Navigate to CloudWatch → Log groups → `/aws/monitoring-dashboard`

### Testing with LocalStack

For local testing without AWS costs:

1. **Start LocalStack:**
   ```bash
   docker run -d -p 4566:4566 localstack/localstack
   ```

2. **Configure endpoint:**
   ```bash
   export CLOUDWATCH_ENDPOINT=http://localhost:4566
   export CLOUDWATCH_METRICS_ENABLED=true
   export CLOUDWATCH_LOGS_ENABLED=true
   export CLOUDWATCH_REGION=us-east-1
   export CLOUDWATCH_ACCESS_KEY_ID=test
   export CLOUDWATCH_SECRET_ACCESS_KEY=test
   ```

3. **Run application:**
   ```bash
   make run
   ```

4. **Verify with AWS CLI:**
   ```bash
   # List metrics
   aws --endpoint-url=http://localhost:4566 cloudwatch list-metrics \
     --namespace MonitoringDashboard/System

   # Get metric statistics
   aws --endpoint-url=http://localhost:4566 cloudwatch get-metric-statistics \
     --namespace MonitoringDashboard/System \
     --metric-name cpu_usage \
     --start-time 2026-02-08T00:00:00Z \
     --end-time 2026-02-08T23:59:59Z \
     --period 300 \
     --statistics Average

   # List log streams
   aws --endpoint-url=http://localhost:4566 logs describe-log-streams \
     --log-group-name /aws/monitoring-dashboard
   ```

## CloudWatch Insights Queries

### Query Logs

Example CloudWatch Logs Insights queries:

```sql
# All error logs
fields timestamp, level, message, fields.error
| filter level = "ERROR"
| sort timestamp desc
| limit 100

# Metrics collection performance
fields timestamp, message, fields.count, fields.duration_ms
| filter message like /Metrics collected/
| stats avg(fields.duration_ms) as avg_duration, max(fields.duration_ms) as max_duration by bin(5m)

# Warning and error summary
fields timestamp, level, message
| filter level in ["WARN", "ERROR"]
| stats count() by level, bin(1h)
```

### Create CloudWatch Dashboard

Example dashboard configuration:

```json
{
  "widgets": [
    {
      "type": "metric",
      "properties": {
        "metrics": [
          ["MonitoringDashboard/System", "cpu_usage", {"stat": "Average"}],
          [".", "memory_usage", {"stat": "Average"}],
          [".", "disk_usage", {"stat": "Average"}]
        ],
        "period": 300,
        "stat": "Average",
        "region": "us-east-1",
        "title": "System Metrics"
      }
    }
  ]
}
```

## Cost Optimization

### CloudWatch Metrics Costs

- **Standard resolution (60s)**: ~$0.30 per metric per month
- **High resolution (1s)**: ~$0.90 per metric per month
- **API calls**: Free tier covers typical usage

**Optimization tips:**
1. Use standard resolution (60s) unless you need sub-minute granularity
2. Increase buffer size to reduce API calls
3. Use dimensions strategically - each unique combination creates a new metric

### CloudWatch Logs Costs

- **Ingestion**: $0.50 per GB
- **Storage**: $0.03 per GB per month
- **Insights queries**: $0.005 per GB scanned

**Optimization tips:**
1. Adjust log level in production (INFO or WARN instead of DEBUG)
2. Use log retention policies to expire old logs
3. Increase flush interval to batch more events per API call
4. Filter logs before publishing (e.g., only publish ERROR level to CloudWatch)

## Monitoring & Observability

### Application Metrics

The application publishes the following metrics about itself:

```
MonitoringDashboard/System
  - cpu_usage (Percent)
  - memory_usage (Percent)
  - disk_usage (Percent)
  - network_sent (Megabytes/Second)
```

### Application Logs

All application logs are streamed to CloudWatch with structured fields for filtering and analysis.

### Graceful Shutdown

On shutdown (SIGTERM/SIGINT), the application:
1. Stops the metrics collector
2. Flushes CloudWatch metrics buffer
3. Flushes CloudWatch logs buffer
4. Shuts down HTTP server

This ensures no data loss during deployments or restarts.

## Troubleshooting

### Metrics not appearing in CloudWatch

**Check:**
1. `CLOUDWATCH_METRICS_ENABLED=true` is set
2. AWS credentials are valid and have CloudWatch permissions
3. Application logs for "CloudWatch metrics publisher initialized"
4. Application logs for any CloudWatch publish errors
5. Namespace name matches in AWS Console

**Required IAM Permissions:**
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "cloudwatch:PutMetricData"
      ],
      "Resource": "*"
    }
  ]
}
```

### Logs not appearing in CloudWatch

**Check:**
1. `CLOUDWATCH_LOGS_ENABLED=true` is set
2. Log group and stream names are correct
3. Application logs for "CloudWatch logs publisher initialized"
4. Auto-create is enabled or log group/stream exist

**Required IAM Permissions:**
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents"
      ],
      "Resource": "arn:aws:logs:*:*:log-group:/aws/monitoring-dashboard:*"
    }
  ]
}
```

### High API costs

**Solutions:**
1. Increase buffer sizes
2. Increase flush intervals
3. Reduce number of custom dimensions
4. Use standard resolution instead of high resolution
5. Disable CloudWatch in non-production environments

## Best Practices

1. **Use IAM Roles**: In production (ECS/EKS), use IAM roles instead of static credentials
2. **Separate environments**: Use different namespaces/log groups per environment
3. **Monitor costs**: Set up billing alerts for CloudWatch usage
4. **Log retention**: Configure retention policies (e.g., 7 days for dev, 30 days for prod)
5. **Dimensions**: Keep dimensions cardinality low to control costs
6. **Graceful degradation**: Application continues working even if CloudWatch fails

## Future Enhancements

- [ ] CloudWatch Alarms integration
- [ ] IAM roles support (instead of static credentials)
- [ ] Custom metric filtering (publish only specific metrics)
- [ ] Log level filtering (publish only WARN/ERROR to CloudWatch)
- [ ] CloudWatch Events/EventBridge integration
- [ ] Distributed tracing with X-Ray
- [ ] Prometheus-compatible metrics exporter

## References

- [AWS CloudWatch Metrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/working_with_metrics.html)
- [AWS CloudWatch Logs](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/WhatIsCloudWatchLogs.html)
- [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/docs/)
- [CloudWatch Pricing](https://aws.amazon.com/cloudwatch/pricing/)
