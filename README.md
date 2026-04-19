# weather-etl

A Go-based ETL pipeline that continuously ingests weather data from [OpenWeatherMap](https://openweathermap.org/api) for multiple cities, persists it to PostgreSQL and a local file-based data lake, and exposes HTTP endpoints for health checks and Prometheus metrics. A full observability stack (Prometheus + Grafana) is included via docker-compose.

---

## Architecture

```
OpenWeatherMap API  (London, Madrid, Prague — concurrent)
       │
       ▼
  [Extractor]  ──── HTTP GET every 30s per city (goroutine per city)
       │
       ├──► data/raw/city=<city>/date=YYYY-MM-DD/data.json   (append every fetch)
       ├──► PostgreSQL: weather_raw                         (insert every fetch)
       │
       ▼
  [Transformer]  ── normalize fields, drop UI-only data, UTC timestamps
       │
       ├──► data/processed/city=<city>/date=YYYY-MM-DD/data.json  (append every fetch)
       ├──► PostgreSQL: weather_processed   (upsert — deduplicate on city + observed_at)
       │
       ▼
  [Observability]
       ├──► logs/etl.log                    (structured JSON via slog)
       ├──► GET /health                     (postgres ping + ETL staleness check)
       ├──► GET /metrics                    (Prometheus)
       ├──► Prometheus  :9090               (scrapes /metrics every 15s)
       └──► Grafana     :3000               (pre-provisioned dashboard)
```

### Data lake vs Data warehouse

| Layer | Storage | Behaviour |
|---|---|---|
| Data lake | `data/raw/city=<city>/date=YYYY-MM-DD/data.json` + `weather_raw` table | Append everything — full fidelity, audit trail |
| Data warehouse | `weather_processed` table | Upsert on `(city, observed_at)` — one row per real OWM observation, no duplicates |

OWM publishes new observations roughly every 10 minutes. The pipeline fetches every 30 seconds, so ~20 fetches map to a single DWH row. The file layer keeps every fetch; the warehouse keeps only meaningful state changes.

### Package layout

| Path | Responsibility |
|---|---|
| `cmd/etl/main.go` | Entry point — wires components, runs the ticker loop |
| `internal/config` | Loads and validates configuration from environment variables |
| `internal/model` | Shared data structs: `RawWeather`, `ProcessedWeather` |
| `internal/extractor` | OpenWeatherMap HTTP client |
| `internal/transformer` | Normalizes raw API response into a clean `ProcessedWeather` struct |
| `internal/loader` | Appends JSON records to daily files under `data/` |
| `internal/storage` | PostgreSQL operations — raw insert, processed upsert |
| `internal/server` | HTTP server: `/health` and `/metrics`, Prometheus metric vars |
| `internal/logger` | `slog`-based structured JSON logger to `logs/etl.log` |

---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/)
- An [OpenWeatherMap API key](https://home.openweathermap.org/api_keys) (free tier is sufficient)

---

## Quick Start

### 1. Clone and configure

```bash
git clone <repo-url>
cd weather-etl
cp .env.example .env
# Edit .env — at minimum set OWM_API_KEY
```

### 2. Start the stack

```bash
docker-compose up --build
```

This starts four services: `postgres`, `app`, `prometheus`, and `grafana`. Postgres runs the schema migration automatically on first boot.

### 3. Verify

```bash
# Health check — returns 200 OK or 503 if postgres/ETL is degraded
curl http://localhost:8080/health

# Prometheus metrics
curl http://localhost:8080/metrics | grep etl_

# Grafana dashboard
open http://localhost:3000  # login: admin / <GRAFANA_ADMIN_PASSWORD>

# Prometheus UI
open http://localhost:9090
```

### 4. Inspect persisted data

```bash
# Local files — Hive-partitioned by city and date
ls data/raw/
ls data/processed/
cat logs/etl.log | head -20

# Raw records (every fetch)
docker-compose exec postgres psql -U $DB_USER -d $DB_NAME \
  -c "SELECT count(*), city FROM weather_raw GROUP BY city;"

# Processed records (one per real OWM observation)
docker-compose exec postgres psql -U $DB_USER -d $DB_NAME \
  -c "SELECT observed_at, ingested_at, city, temperature_celsius, weather_description \
      FROM weather_processed ORDER BY observed_at DESC LIMIT 10;"
```

---

## Configuration

Copy `.env.example` to `.env`. All variables are read at startup.

| Variable | Default | Description |
|---|---|---|
| `OWM_API_KEY` | *(required)* | OpenWeatherMap API key |
| `OWM_CITIES` | `London,Madrid,Prague` | Comma-separated list of cities to fetch |
| `OWM_UNITS` | `metric` | Unit system (`metric` = Celsius, `imperial` = Fahrenheit) |
| `DB_USER` | `etl` | Postgres username |
| `DB_PASSWORD` | *(required)* | Postgres password |
| `DB_NAME` | `weather` | Postgres database name |
| `HTTP_PORT` | `8080` | Port for `/health` and `/metrics` |
| `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `FETCH_INTERVAL_SECONDS` | `30` | How often to poll the API per city |
| `GRAFANA_ADMIN_PASSWORD` | *(required)* | Grafana admin login password |

---

## Data Model

### Raw (`data/raw/city=<city>/date=YYYY-MM-DD/data.json` + `weather_raw` table)

The full OpenWeatherMap API response, stored as-is with an added `ingested_at` timestamp marking when the pipeline fetched it. Useful for reprocessing, debugging, and auditing exactly what the upstream API returned.

### Processed (`data/processed/city=<city>/date=YYYY-MM-DD/data.json` + `weather_processed` table)

A normalized, analytics-friendly record. OWM-internal IDs, icon codes, and sunrise/sunset are dropped. All units are SI.

```json
{
  "observed_at":          "2024-01-15T10:30:00Z",
  "ingested_at":          "2024-01-15T10:30:01Z",
  "city":                 "London",
  "country":              "GB",
  "temperature_celsius":  8.5,
  "feels_like_celsius":   6.1,
  "humidity_percent":     82,
  "pressure_hpa":         1012,
  "weather_condition":    "Clouds",
  "weather_description":  "overcast clouds",
  "wind_speed_ms":        4.6,
  "wind_direction_deg":   250,
  "visibility_m":         10000,
  "cloudiness_percent":   100
}
```

**`observed_at`** — when OWM measured the data (from the `dt` field in the API response, converted to UTC ISO 8601).

**`ingested_at`** — when the pipeline fetched it. The gap between the two reflects OWM's own publication lag.

### File storage layout

```
data/
├── raw/
│   ├── city=london/date=2024-01-15/data.json
│   ├── city=madrid/date=2024-01-15/data.json
│   └── city=prague/date=2024-01-15/data.json
└── processed/
    ├── city=london/date=2024-01-15/data.json
    ├── city=madrid/date=2024-01-15/data.json
    └── city=prague/date=2024-01-15/data.json
```

Each city writes to its own file, eliminating concurrent write conflicts. Partition directories are created on first write and rotate at UTC midnight. Both layers append every cycle; deduplication only applies to the Postgres DWH table.

### DWH deduplication

`weather_processed` has a unique constraint on `(city, observed_at)`. The insert uses `ON CONFLICT (city, observed_at) DO NOTHING RETURNING id` — if the observation already exists, the row is skipped and counted in `etl_records_skipped_total`.

---

## Observability

### Health endpoint

`GET /health` performs live checks and returns HTTP 200 when healthy or HTTP 503 when degraded.

```json
{
  "status": "ok",
  "uptime_seconds": 3600,
  "last_etl_cycle_seconds_ago": 12.4,
  "checks": {
    "postgres": "ok",
    "etl": "ok"
  }
}
```

ETL status becomes `stale` (and the response 503) if no successful cycle has completed in the last 2 minutes.

### Logging

Structured JSON logs written to `logs/etl.log` via `log/slog`:

```json
{"time":"2024-01-15T10:30:01Z","level":"INFO","msg":"api request succeeded","city":"London","latency_ms":212}
{"time":"2024-01-15T10:30:01Z","level":"INFO","msg":"processed record skipped as duplicate","city":"London","observed_at":"2024-01-15T10:20:00Z"}
{"time":"2024-01-15T10:30:01Z","level":"INFO","msg":"etl cycle completed successfully","city":"Madrid"}
```

### Prometheus Metrics

Available at `GET /metrics` and scraped by Prometheus at `:9090`.

| Metric | Type | Labels | Description |
|---|---|---|---|
| `etl_api_requests_total` | Counter | `status=success\|failure` | Total API calls — rate the failure label to alert on city fetch errors |
| `etl_api_latency_seconds` | Histogram | — | OWM response time — p95/p99 spikes indicate upstream degradation |
| `etl_transform_errors_total` | Counter | — | Any value > 0 means OWM changed their response schema |
| `etl_records_saved_total` | Counter | `type=raw\|processed`, `backend=file\|postgres` | Records actually written |
| `etl_records_skipped_total` | Counter | `type=processed`, `backend=postgres` | DWH upserts skipped as duplicates — expected to be high between OWM updates |
| `etl_last_successful_run_timestamp` | Gauge | — | Alert if `time() - etl_last_successful_run_timestamp > 120` |

### Grafana Dashboard

Pre-provisioned at `http://localhost:3000` under **Dashboards → Weather ETL**.

| Panel | Purpose |
|---|---|
| Pipeline Status | Seconds since last successful cycle — green/yellow/red thresholds |
| API Errors/sec | Stat panel — turns red the moment any city fetch fails |
| API Requests/sec | Success vs failure rate over time |
| API Latency Percentiles | p50 / p95 / p99 |
| Records Saved/sec | Data flowing through all backends |
| DWH Deduplication | Saved (green) vs skipped (orange) for `weather_processed` — spikes every ~10 min confirm OWM cadence |
| Transform Errors/sec | Schema breakage detector |

---

## Tests

```bash
go test ./...
```

Transformer tests cover: nil input, field mapping correctness, UTC timestamp normalization, and graceful handling of an empty `weather` array from the API.

---

## Productionization

### Infrastructure

| Component | Local | Production |
|---|---|---|
| API ingestion | Single container | Kubernetes Deployment or AWS ECS Fargate task |
| OLTP database | Postgres in docker-compose | AWS RDS PostgreSQL (Multi-AZ, automated backups) |
| OLAP database | — | Amazon Redshift or Snowflake (hourly/daily sync from RDS) |
| Message broker | — | Apache Kafka (MSK) or AWS SQS |
| File storage | Local `data/` mount | AWS S3 (Parquet, partitioned by `city/year/month/day/`) |
| Orchestration | `time.Ticker` in-process | Apache Airflow (MWAA) or AWS Step Functions |
| Secrets | `.env` file | AWS Secrets Manager / HashiCorp Vault |
| Metrics | Prometheus + Grafana containers | AWS Managed Prometheus + Grafana, or Datadog |
| Logs | `logs/etl.log` | CloudWatch Logs / Datadog |
| Container registry | Local build | AWS ECR |

---

### OLTP vs OLAP

The pipeline currently writes everything to PostgreSQL, which is an **OLTP** (Online Transaction Processing) database: row-oriented storage, strong ACID guarantees, optimized for high-frequency small writes and point lookups with low latency. This is the right choice for the real-time operational path — each 30-second fetch produces one or two small rows and we need sub-millisecond upserts.

However, PostgreSQL is a poor fit for analytical workloads (e.g. "average temperature per city per hour over the last 6 months"). As the dataset grows, full-scan aggregations become expensive, and analytics queries contend with the write path on the same hardware.

The production answer is a two-tier storage strategy:

| | OLTP (PostgreSQL / RDS) | OLAP (Redshift / Snowflake) |
|---|---|---|
| Storage model | Row-oriented | Columnar — reads compress and scan only the columns needed |
| Write pattern | Frequent small inserts/upserts | Bulk loads (COPY, microbatch every hour or daily) |
| Query pattern | Point lookups, recent rows | Aggregations, window functions, cross-city time-series |
| Latency | Sub-millisecond | Seconds to minutes (acceptable for analytics) |
| Deduplication | `ON CONFLICT DO NOTHING` at write time | Handled at load time or via `MERGE` statement |
| Use cases | Health checks, real-time dashboards, operational queries | BI tools, historical analysis, trend detection |

Sync strategy: an Airflow DAG (or Step Functions state machine) runs hourly and bulk-inserts new `weather_processed` rows from RDS into Redshift using a high-watermark on `ingested_at`. The RDS table remains the system of record; Redshift is a read-optimized replica for analytics.

---

### Pipeline Decomposition (Producer / Consumer)

The current pipeline couples all stages in a single process: fetch → transform → save raw → save processed. In production these should be decoupled into independent services so each stage can fail, retry, and scale independently.

```
[Fetcher service]
  OWM API → Kafka topic: weather.raw
                │
                ▼
[Transformer service]  (consumer group)
  weather.raw → transform → Kafka topic: weather.processed
                                    │
                          ┌─────────┴─────────┐
                          ▼                   ▼
                   [Loader: S3]        [Loader: Postgres]
                  raw/ + processed/    weather_raw + weather_processed
```

**Why Kafka here:**
- **Decoupling**: the fetcher does not block waiting for Postgres or S3 to finish.
- **Replay**: if the transformer has a bug and is deployed with a fix, the raw topic can be re-consumed from an earlier offset to reprocess historical data without re-fetching from OWM.
- **Backpressure**: if Postgres is slow, messages queue in Kafka rather than causing cascading timeouts upstream.
- **Multiple consumers**: an additional consumer can subscribe to `weather.processed` independently — e.g. a service that triggers alerts when temperature drops below freezing, without touching the main pipeline.
- **Dead-letter topic**: failed transform or load events are routed to `weather.dlq` for reprocessing rather than silently dropped.

For lower operational complexity at smaller scale, **AWS SQS** (+ SNS fan-out) is a viable alternative: fetcher → SNS → two SQS queues (one per downstream consumer). No replay capability, but zero broker management.

---

### Orchestration with Airflow

The `time.Ticker` approach works for a continuously-running daemon but has no visibility into historical run history, no built-in retry with backoff across runs, and no dependency management between tasks.

Benefits:
- **Backfill**: re-run historical dates after a bug fix with `airflow dags backfill`.
- **SLA tracking**: alert if a task takes >90s (OWM normally responds in <500ms — a slow run signals upstream issues).
- **Task-level retries**: `retries=3, retry_delay=timedelta(seconds=10)` per task, independent of other tasks.
- **Dependency management**: `sync_to_redshift` only runs after all save tasks succeed.
- **Audit trail**: every run, its duration, and its status is stored in Airflow's metadata DB.

AWS MWAA (Managed Workflows for Apache Airflow) removes the need to self-manage the Airflow scheduler and workers.

---

### S3 Partitioning and Predicate Pushdown

Raw and processed files already use a Hive-style partition layout locally. In production, write Parquet files to S3 with the same structure, extended with sub-day partitions:

```
s3://bucket/processed/city=London/year=2024/month=01/day=15/HH-MM.parquet
s3://bucket/processed/city=Madrid/year=2024/month=01/day=15/HH-MM.parquet
```

**Why this matters — partition pruning:**

When Athena or Glue runs `SELECT avg(temperature_celsius) FROM processed WHERE city = 'London' AND year = 2024 AND month = 1`, the query engine reads the S3 partition metadata first and **skips all S3 objects that do not match the predicate**. It never fetches the Madrid or Prague partitions at all — no data is transferred, and the query costs a fraction of a full-dataset scan.

**Column-level predicate pushdown (Parquet):**

Parquet stores per-row-group min/max statistics for each column. For a query like `WHERE temperature_celsius > 20`, the engine reads the footer statistics and **skips entire row groups** where `max(temperature_celsius) ≤ 20`. With good sort order (e.g. files sorted by `observed_at`), this eliminates most I/O for time-range queries.

**Practical impact:** a query on one city for one month over 6 months of global data goes from scanning ~180 files to scanning ~30 files (one per day for that city/month), with further row-group skips inside each file.

Glue Crawlers keep the partition catalog up to date automatically as new files land in S3.

---

### Scalability

- **More cities**: `OWM_CITIES` already accepts a comma-separated list. Each city gets its own goroutine within each tick.
- **Higher concurrency**: Replace the single ticker with a worker pool. Cities are dispatched to a job channel; N workers process them in parallel with a configurable concurrency limit.
- **Read replicas**: Point analytics queries at an RDS read replica to avoid contention with the write path.
- **Horizontal scaling**: With Kafka in place, multiple fetcher instances can share city assignments; multiple transformer replicas form a consumer group and Kafka distributes partitions across them.

---

### Reliability

- **Retries with backoff**: Wrap `client.Fetch()` with exponential backoff + jitter (e.g. `github.com/cenkalti/backoff`). Currently a failed fetch skips that city for the cycle.
- **Dead-letter queue**: Failed inserts are logged but dropped. In production, route to SQS DLQ or a Kafka DLQ topic for reprocessing.
- **Schema migrations**: Replace the init SQL with `golang-migrate` for versioned forward/rollback migrations.
- **Graceful shutdown**: Already implemented via `SIGTERM` handling — pairs well with Kubernetes `preStop` hooks.
- **Alerting**: Alertmanager rule on `time() - etl_last_successful_run_timestamp > 120` to page on-call if the pipeline stalls.

---

### CI/CD

GitHub Actions pipeline: lint (`golangci-lint`) → `go test ./...` → build Docker image → push to ECR → deploy to ECS/K8s. Infrastructure provisioned separately via Terraform or AWS CDK.
