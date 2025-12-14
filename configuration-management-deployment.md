# Configuration and Deployment Strategies

## Configuration Philosophy

**Key Principles:**
1. **12-Factor App compliance**: Secrets via env vars, config via files
2. **Layered overrides**: Defaults → Config file → Env vars
3. **Service-specific + shared**: Common DB config, service-specific tuning
4. **Validation at startup**: Fail fast with clear error messages
5. **No runtime reloads**: Restart to reconfigure (simplicity over hot-reload complexity)

---

## Directory Structure

```
osrs-prices/
├── configs/
│   ├── base.yaml              # Shared defaults
│   ├── poller.yaml            # Poller-specific overrides
│   ├── backfill.yaml          # Backfill-specific
│   ├── gap-filler.yaml        # Gap-filler-specific
│   └── api-server.yaml        # API server config
├── docker-compose.yaml
├── .env.example               # Template for secrets
└── cmd/
    └── */main.go              # Each binary loads config
```

---

## Configuration Schema

### **base.yaml** (Shared across all services)

```yaml
# Database configuration
database:
  host: postgres
  port: 5432
  name: osrs_prices
  user: osrs_app
  # password: from env var DB_PASSWORD
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m
  connect_timeout: 10s

# TimescaleDB specific
timescaledb:
  chunk_time_interval: 7d      # Partition size for price_observations
  retention_policy:
    raw_observations: 30d      # Keep detailed data for 30 days
    # Buckets kept indefinitely

# Upstream API configuration
osrs_api:
  base_url: "https://prices.runescape.wiki/api/v1/osrs"
  timeout: 30s
  retry:
    max_attempts: 3
    initial_backoff: 1s
    max_backoff: 30s
    backoff_multiplier: 2.0

# Rate limiting (shared budget)
rate_limit:
  strategy: "local"            # local|redis|database
  requests_per_minute: 600
  burst_capacity: 20
  # Redis config (if strategy=redis)
  redis:
    host: redis
    port: 6379
    # password: from env var REDIS_PASSWORD
    db: 0

# Observability
logging:
  level: info                  # debug|info|warn|error
  format: json                 # json|console
  output: stdout

metrics:
  enabled: true
  port: 9090
  path: /metrics

# Operational metadata
system:
  instance_id: ""              # Auto-generated if empty (hostname + random suffix)
  ingestion_log_enabled: true  # Track all ingestion attempts
```

### **poller.yaml** (Poller-specific)

```yaml
poller:
  # Polling interval for /latest endpoint
  interval: 60s

  # Jitter to avoid thundering herd if multiple instances
  interval_jitter: 5s

  # Timestamp precision for observations
  timestamp_precision: microsecond  # second|millisecond|microsecond

  # Batch size for inserts
  batch_size: 1000                  # Insert 1000 items per transaction

  # Health check - fail if no successful poll in this window
  health_check_timeout: 5m

  # Whether to track high_time/low_time changes
  track_price_timestamps: true

# Lifecycle settings (poller can optionally prune old data)
lifecycle:
  enabled: false                    # Let gap-filler handle this
  prune_interval: 24h
  prune_before: 30d
```

### **backfill.yaml** (Backfill-specific)

```yaml
backfill:
  # Backfill strategy
  mode: historical                  # historical|item_list|date_range

  # Historical mode: backfill all items for time range
  historical:
    start_date: "2024-01-01T00:00:00Z"
    end_date: "now"                 # Special keyword for current time
    timesteps:
      - 24h
      - 1h
      - 5m

  # Item list mode: specific items only
  item_list:
    items: []                       # [4151, 4153, 11802, ...]
    timesteps: [24h, 1h, 5m]

  # Date range mode: all items for specific range
  date_range:
    start: ""
    end: ""
    timesteps: [5m]

  # Execution parameters
  concurrency: 5                    # Process 5 items concurrently
  checkpoint_interval: 100          # Save progress every 100 items
  checkpoint_file: /data/backfill_checkpoint.json

  # Rate limiting (backfill-specific overrides)
  rate_limit:
    requests_per_minute: 300        # More conservative than poller
    burst_capacity: 10

  # Retry strategy for failed items
  retry:
    enabled: true
    max_retries: 3
    failed_items_file: /data/backfill_failed.json

  # Skip items that already have data
  skip_existing: true

  # Locking to prevent concurrent backfills
  lock:
    enabled: true
    mechanism: advisory_lock        # advisory_lock|file|redis
    lock_id: 123456                 # PostgreSQL advisory lock ID
    timeout: 2h                     # Fail if can't acquire lock in 2h
```

### **gap-filler.yaml** (Gap-filler-specific)

```yaml
gap_filler:
  # Execution mode
  mode: scheduled                   # scheduled|oneshot|continuous

  # Scheduled mode
  schedule:
    check_interval: 1h              # Run every hour
    check_window: 24h               # Look back 24 hours for gaps

  # Gap detection
  detection:
    bucket_sizes: [5m, 1h, 24h]
    # Only check for gaps in recent data
    min_age: 5m                     # Don't check buckets newer than 5m
    max_age: 7d                     # Don't check buckets older than 7d

    # Items to check (empty = all items)
    item_filter: []

    # Expected data source per time range
    expectations:
      - age_range: "0-24h"
        source: api                 # Expect API data
        bucket_size: 5m
      - age_range: "24h-7d"
        source: any                 # API or computed is fine
        bucket_size: 1h

  # Gap filling strategies
  filling:
    # Try API first, fall back to computing from raw data
    strategies:
      - type: api
        endpoints:
          5m: /5m
          1h: /timeseries
          24h: /timeseries
        max_gaps_per_run: 100       # Limit API calls per run

      - type: compute
        source_table: price_observations
        min_observations: 3         # Need at least 3 obs to compute bucket
        fallback_value: null        # If insufficient data, write null

    # Prioritization
    priority: newest_first          # newest_first|oldest_first|random

  # Lifecycle management (run after gap filling)
  lifecycle:
    enabled: true

    # Drop old raw observations
    prune_observations:
      enabled: true
      older_than: 30d

    # Vacuum/analyze after large deletes
    maintenance:
      enabled: true
      threshold: 10000              # Run if deleted >10k rows

  # Coordination with backfill
  coordination:
    check_backfill_status: true
    skip_if_backfill_running: true
    backfill_detection:
      method: advisory_lock         # advisory_lock|ingestion_log|redis
      lock_id: 123456               # Same as backfill lock_id
```

### **api-server.yaml** (Future API server)

```yaml
api_server:
  host: 0.0.0.0
  port: 8080

  # CORS
  cors:
    enabled: true
    allowed_origins: ["*"]
    allowed_methods: [GET, POST]

  # Rate limiting (per-client)
  rate_limit:
    enabled: true
    requests_per_minute: 60
    burst: 10

  # Caching
  cache:
    enabled: true
    ttl: 60s
    max_size: 1000                  # Cache up to 1000 queries

  # Query limits
  limits:
    max_items_per_query: 100
    max_time_range: 365d
    default_bucket_size: 1h
```

---

## Configuration Loading in Go

### **Shared config package**

```go
// internal/config/config.go
package config

import (
    "fmt"
    "os"
    "time"

    "gopkg.in/yaml.v3"
)

// BaseConfig is shared across all services
type BaseConfig struct {
    Database    DatabaseConfig    `yaml:"database"`
    TimescaleDB TimescaleDBConfig `yaml:"timescaledb"`
    OSRSApi     OSRSApiConfig     `yaml:"osrs_api"`
    RateLimit   RateLimitConfig   `yaml:"rate_limit"`
    Logging     LoggingConfig     `yaml:"logging"`
    Metrics     MetricsConfig     `yaml:"metrics"`
    System      SystemConfig      `yaml:"system"`
}

type DatabaseConfig struct {
    Host            string        `yaml:"host"`
    Port            int           `yaml:"port"`
    Name            string        `yaml:"name"`
    User            string        `yaml:"user"`
    Password        string        `yaml:"-"` // Never in file, always from env
    MaxOpenConns    int           `yaml:"max_open_conns"`
    MaxIdleConns    int           `yaml:"max_idle_conns"`
    ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
    ConnectTimeout  time.Duration `yaml:"connect_timeout"`
}

type RateLimitConfig struct {
    Strategy           string      `yaml:"strategy"`
    RequestsPerMinute  int         `yaml:"requests_per_minute"`
    BurstCapacity      int         `yaml:"burst_capacity"`
    Redis              RedisConfig `yaml:"redis"`
}

type RedisConfig struct {
    Host     string `yaml:"host"`
    Port     int    `yaml:"port"`
    Password string `yaml:"-"` // From env
    DB       int    `yaml:"db"`
}

// ... other config structs

// Load loads base config and applies env var overrides
func Load(configPaths ...string) (*BaseConfig, error) {
    cfg := &BaseConfig{}

    // Load and merge YAML files
    for _, path := range configPaths {
        if err := loadYAML(path, cfg); err != nil {
            return nil, fmt.Errorf("loading %s: %w", path, err)
        }
    }

    // Apply environment variable overrides
    if err := applyEnvOverrides(cfg); err != nil {
        return nil, fmt.Errorf("applying env overrides: %w", err)
    }

    // Validate
    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    return cfg, nil
}

func loadYAML(path string, cfg *BaseConfig) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    return yaml.Unmarshal(data, cfg)
}

func applyEnvOverrides(cfg *BaseConfig) error {
    // Secrets from environment
    if pw := os.Getenv("DB_PASSWORD"); pw != "" {
        cfg.Database.Password = pw
    }
    if pw := os.Getenv("REDIS_PASSWORD"); pw != "" {
        cfg.RateLimit.Redis.Password = pw
    }

    // Optional overrides
    if host := os.Getenv("DB_HOST"); host != "" {
        cfg.Database.Host = host
    }
    if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
        cfg.Logging.Level = logLevel
    }

    // Instance ID
    if cfg.System.InstanceID == "" {
        hostname, _ := os.Hostname()
        cfg.System.InstanceID = fmt.Sprintf("%s-%d", hostname, time.Now().Unix())
    }

    return nil
}

func (c *BaseConfig) Validate() error {
    if c.Database.Host == "" {
        return fmt.Errorf("database.host is required")
    }
    if c.Database.Password == "" {
        return fmt.Errorf("DB_PASSWORD environment variable is required")
    }
    if c.RateLimit.RequestsPerMinute <= 0 {
        return fmt.Errorf("rate_limit.requests_per_minute must be > 0")
    }
    // ... more validations
    return nil
}
```

### **Service-specific config**

```go
// cmd/ingest-backfill/config.go
package main

import (
    "osrs-prices/internal/config"
)

type BackfillConfig struct {
    *config.BaseConfig
    Backfill BackfillSettings `yaml:"backfill"`
}

type BackfillSettings struct {
    Mode              string             `yaml:"mode"`
    Historical        HistoricalMode     `yaml:"historical"`
    Concurrency       int                `yaml:"concurrency"`
    CheckpointFile    string             `yaml:"checkpoint_file"`
    SkipExisting      bool               `yaml:"skip_existing"`
    Lock              LockConfig         `yaml:"lock"`
}

// ... other structs

func LoadBackfillConfig() (*BackfillConfig, error) {
    // Load base + service-specific
    base, err := config.Load("configs/base.yaml", "configs/backfill.yaml")
    if err != nil {
        return nil, err
    }

    cfg := &BackfillConfig{BaseConfig: base}

    // Load backfill-specific from the merged config
    // (yaml.Unmarshal already populated it during Load)

    return cfg, nil
}
```

---

## Docker Compose Configuration

```yaml
# docker-compose.yaml
version: '3.8'

services:
  postgres:
    image: timescale/timescaledb:latest-pg16
    environment:
      POSTGRES_DB: osrs_prices
      POSTGRES_USER: osrs_app
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U osrs_app"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass ${REDIS_PASSWORD}
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s

  poller:
    build:
      context: .
      dockerfile: Dockerfile
      target: poller
    environment:
      - DB_PASSWORD=${DB_PASSWORD}
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - LOG_LEVEL=${LOG_LEVEL:-info}
    volumes:
      - ./configs:/app/configs:ro
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    restart: unless-stopped
    ports:
      - "9091:9090"  # Metrics

  backfill:
    build:
      context: .
      dockerfile: Dockerfile
      target: backfill
    environment:
      - DB_PASSWORD=${DB_PASSWORD}
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - LOG_LEVEL=${LOG_LEVEL:-info}
    volumes:
      - ./configs:/app/configs:ro
      - backfill-data:/data  # For checkpoints
    depends_on:
      postgres:
        condition: service_healthy
    profiles:
      - backfill  # Only run when explicitly requested
    ports:
      - "9092:9090"

  gap-filler:
    build:
      context: .
      dockerfile: Dockerfile
      target: gap-filler
    environment:
      - DB_PASSWORD=${DB_PASSWORD}
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - LOG_LEVEL=${LOG_LEVEL:-info}
    volumes:
      - ./configs:/app/configs:ro
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped
    ports:
      - "9093:9090"

  api-server:
    build:
      context: .
      dockerfile: Dockerfile
      target: api-server
    environment:
      - DB_PASSWORD=${DB_PASSWORD}
      - LOG_LEVEL=${LOG_LEVEL:-info}
    volumes:
      - ./configs:/app/configs:ro
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped
    ports:
      - "8080:8080"  # API
      - "9094:9090"  # Metrics

volumes:
  postgres-data:
  backfill-data:
```

### **.env.example**

```bash
# Copy to .env and fill in secrets
DB_PASSWORD=changeme_secure_password
REDIS_PASSWORD=changeme_redis_password

# Optional overrides
LOG_LEVEL=info
DB_HOST=postgres

# Service-specific (uncomment to override YAML)
# POLLER_INTERVAL=60s
# BACKFILL_CONCURRENCY=5
# GAP_FILLER_CHECK_INTERVAL=1h
```

---

## Dockerfile (Multi-stage)

```dockerfile
# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build each binary
RUN CGO_ENABLED=0 go build -o /bin/poller ./cmd/ingest-poller
RUN CGO_ENABLED=0 go build -o /bin/backfill ./cmd/ingest-backfill
RUN CGO_ENABLED=0 go build -o /bin/gap-filler ./cmd/gap-filler
RUN CGO_ENABLED=0 go build -o /bin/api-server ./cmd/api-server

# Poller image
FROM alpine:latest AS poller
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /bin/poller /app/poller
WORKDIR /app
ENTRYPOINT ["/app/poller"]

# Backfill image
FROM alpine:latest AS backfill
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /bin/backfill /app/backfill
WORKDIR /app
ENTRYPOINT ["/app/backfill"]

# Gap-filler image
FROM alpine:latest AS gap-filler
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /bin/gap-filler /app/gap-filler
WORKDIR /app
ENTRYPOINT ["/app/gap-filler"]

# API server image
FROM alpine:latest AS api-server
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /bin/api-server /app/api-server
WORKDIR /app
ENTRYPOINT ["/app/api-server"]
```

---

## Usage Examples

**Start everything:**
```bash
docker-compose up -d
```

**Run backfill once:**
```bash
docker-compose run --rm backfill
```

**Override config for testing:**
```bash
LOG_LEVEL=debug POLLER_INTERVAL=10s docker-compose up poller
```

**View backfill checkpoint:**
```bash
docker-compose exec backfill cat /data/backfill_checkpoint.json
```

---

## Evolution Path to Kubernetes

This structure maps cleanly to k8s:

```yaml
# k8s/deployment-poller.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: osrs-poller
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: poller
        image: osrs-prices/poller:latest
        env:
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: osrs-secrets
              key: db-password
        volumeMounts:
        - name: config
          mountPath: /app/configs
      volumes:
      - name: config
        configMap:
          name: osrs-configs
```

**ConfigMap from YAML:**
```bash
kubectl create configmap osrs-configs \
  --from-file=configs/base.yaml \
  --from-file=configs/poller.yaml
```

---
