# url-shortener

A distributed URL shortener with a region-aware, coordination-free ID generator and a token-bucket rate limiter (in-memory and Redis-backed implementations). See [DESIGN.md](DESIGN.md) for the full design rationale.

## Prerequisites

- Go 1.26+
- Docker + Docker Compose

## Running locally

Run the whole stack (API + both Postgres shards + Redis) with Docker Compose:

```
docker compose -f deployments/compose/docker-compose.yml up --build
```

The API is then available at `http://localhost:3000`, e.g.:

```
curl -X POST http://localhost:3000/url -d '{"long_url":"https://example.com"}'
curl -i http://localhost:3000/url/<short_id>
curl http://localhost:3000/healthz
```

Alternatively, run just the infrastructure and the API locally (faster iteration, no rebuild step):

```
docker compose -f deployments/compose/docker-compose.yml up -d postgres_region1 postgres_region2 redis-cache
NODE_ID=1 REGION_ID=1 go run ./cmd/api
```

## Running tests

```
go test ./...                       # unit tests, no infrastructure required
go test -tags=integration ./...     # integration tests, requires the compose stack running
```

Both suites also run with `-race` in CI.

## Configuration

All configuration is read from environment variables by `internal/config`, with defaults suitable for local development. `NODE_ID` and `REGION_ID` have no default and must always be set.

| Variable | Default | Description |
|---|---|---|
| `PORT` | `3000` | HTTP port the API listens on |
| `NODE_ID` | *required* | This instance's node ID, used in short-ID generation |
| `REGION_ID` | *required* | This instance's region ID; must match a key in the configured Postgres DSNs |
| `REDIS_ADDR` | `localhost:6379` | Redis address, used for caching and rate limiting |
| `POSTGRES_DSN_REGION_1` | `postgres://admin:admin@localhost:5432/shard_region_1?sslmode=disable` | Region-1 Postgres shard DSN |
| `POSTGRES_DSN_REGION_2` | `postgres://admin:admin@localhost:5433/shard_region_2?sslmode=disable` | Region-2 Postgres shard DSN |
| `CONNECT_TIMEOUT` | `5s` | Per-attempt timeout when connecting to Redis/Postgres |
| `CONNECT_RETRY_ATTEMPTS` | `5` | Connection retry attempts before giving up at startup |
| `CONNECT_RETRY_BACKOFF` | `5s` | Delay between connection retry attempts |
| `REPOSITORY_QUERY_TIMEOUT` | `5s` | Per-query timeout against Postgres |
| `MAX_REQUEST_BODY_BYTES` | `16384` | Max accepted `POST /url` request body size |
| `DEFAULT_LINK_LIFETIME` | `720h` (30 days) | How long a newly created short link stays valid |
| `CORS_ALLOWED_ORIGINS` | `https://*,http://*` | Comma-separated list of allowed CORS origins |
| `RATE_LIMIT_POST_MAX_TOKENS` | `10` | Token bucket capacity for `POST /url` (per client IP) |
| `RATE_LIMIT_POST_REFILL_RATE` | `1.0` | Token bucket refill rate (tokens/second) for `POST /url` |
| `RATE_LIMIT_GET_IP_MAX_TOKENS` | `100` | Token bucket capacity for `GET /url/{id}` (per client IP) |
| `RATE_LIMIT_GET_IP_REFILL_RATE` | `5.0` | Token bucket refill rate for `GET /url/{id}` (per IP) |
| `RATE_LIMIT_GET_LINK_MAX_TOKENS` | `50` | Token bucket capacity for `GET /url/{id}` (per short link) |
| `RATE_LIMIT_GET_LINK_REFILL_RATE` | `2.0` | Token bucket refill rate for `GET /url/{id}` (per short link) |

## Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/url` | Create a short URL from `{"long_url": "..."}` |
| `GET` | `/url/{id}` | Redirect to the original long URL |
| `GET` | `/healthz` | Health check — `200` when Redis and both Postgres shards are reachable, `503` otherwise |
