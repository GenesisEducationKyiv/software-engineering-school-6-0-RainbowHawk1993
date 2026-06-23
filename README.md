# GitHub Release Notification API

Modular Go application for email subscriptions to GitHub repository releases. The **API service** validates repositories, manages subscriptions in PostgreSQL, and exposes REST and gRPC. The **Scanner microservice** polls GitHub for new releases and sends notification emails.

## Stack

- Go with `chi`
- gRPC alongside REST (public + internal)
- PostgreSQL (API service)
- Redis for GitHub API caching
- Docker / Docker Compose (two services: `api` + `scanner`)
- Mailpit for local SMTP capture

## Architecture

The codebase follows a domain-module layout:

- `internal/modules/subscription` — subscription lifecycle
- `internal/modules/scanner` — release polling and notifications
- `internal/modules/github` — GitHub client + Redis cache
- `internal/modules/notification` — email templates and SMTP
- `cmd/api` — API service (REST, public gRPC, internal gRPC)
- `cmd/scanner` — background scanner worker

See [docs/architecture.md](docs/architecture.md) and [ADR 0004](docs/adr/0004-modular-architecture-and-scanner-microservice.md) for details.

## Run Locally

```bash
docker compose up --build
```

Services:

- API: `http://localhost:8080`
- Prometheus metrics: `http://localhost:8080/metrics`
- Public gRPC: `localhost:9090`
- Internal gRPC: `localhost:9091` (scanner ↔ API)
- Scanner: background worker (no HTTP port)
- Mailpit UI: `http://localhost:8025`
- PostgreSQL: `localhost:5435`
- Redis: internal service `redis:6379`

The HTML UI at `http://localhost:8080` lets you manage subscriptions in the browser. Default API key: `dev-api-key`.

## Environment

Copy `.env.example` to `.env` for custom settings.

**API service:**

- `DATABASE_URL`
- `PORT`, `GRPC_PORT`, `INTERNAL_GRPC_PORT`
- `API_KEY`, `APP_BASE_URL`, `GITHUB_TOKEN`
- `SMTP_*`, `REDIS_*`

**Scanner service:**

- `SUBSCRIPTION_API_GRPC_ADDR` (default `api:9091`)
- `SCAN_INTERVAL`
- `API_KEY`, `APP_BASE_URL`, `GITHUB_TOKEN`
- `SMTP_*`, `REDIS_*`

## API

- `POST /api/subscribe`
- `GET /api/confirm/{token}`
- `GET /api/unsubscribe/{token}`
- `GET /api/subscriptions?email={email}`
- `GET /ui/subscriptions?email={email}`

All REST endpoints above require the `X-API-Key` header.

Example:

```bash
curl -X POST http://localhost:8080/api/subscribe \
  -H 'X-API-Key: dev-api-key' \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","repo":"microsoft/vscode"}'
```

## gRPC

Public service on `localhost:9090` with server reflection. All methods require `x-api-key` metadata.

```bash
grpcurl -plaintext \
  -H 'x-api-key: dev-api-key' \
  -d '{"email":"user@example.com","repo":"microsoft/vscode"}' \
  localhost:9090 releasesapi.v1.SubscriptionService/Subscribe
```

## Development Notes

- Database migrations run on API startup only.
- `last_seen_tag` is seeded from the current latest release at subscribe time.
- Only confirmed subscriptions are scanned.
- GitHub responses are cached in Redis for 10 minutes.
- Scanner has no direct database access; it uses internal gRPC on the API service.

## Testing

```bash
make test
make integration-test-clean
make e2e-test-clean
```

Unit tests:

```bash
go test ./...
```
