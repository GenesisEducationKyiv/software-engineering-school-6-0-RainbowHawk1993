# GitHub Release Notification API

Single-service Go API for email subscriptions to GitHub repository releases. The service validates repositories through the GitHub API, stores subscriptions in PostgreSQL, sends confirmation and release emails through SMTP, and scans for new releases on a fixed interval. (15 minutes by default)

## Stack

- Go with `chi`
- gRPC alongside REST
- PostgreSQL
- Redis for GitHub API caching
- Docker / Docker Compose
- Mailpit for local SMTP capture

## Run Locally

```bash
docker compose up --build
```

Services:

- API: `http://localhost:8080`
- Prometheus metrics: `http://localhost:8080/metrics`
- gRPC: `localhost:9090`
- Mailpit UI: `http://localhost:8025`
- PostgreSQL: `localhost:5435`
- Redis: internal service `redis:6379`

The project serves a small HTML UI at `http://localhost:8080` for creating, viewing, and unsubscribing subscriptions in the browser.

You will have to provide api key on webpage for requests to go through. By default it is set to `dev-api-key`.

## Environment

Copy `.env.example` to `.env` if you need custom settings. Important variables:

- `DATABASE_URL`
- `GRPC_PORT`
- `API_KEY`
- `APP_BASE_URL`
- `GITHUB_TOKEN`
- `SCAN_INTERVAL`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_FROM`
- `REDIS_ADDR`
- `REDIS_PASSWORD`
- `REDIS_DB`

## API

- `POST /api/subscribe`
- `GET /api/confirm/{token}`
- `GET /api/unsubscribe/{token}`
- `GET /api/subscriptions?email={email}`
- `GET /ui/subscriptions?email={email}` returns browser-oriented subscription data used by the HTML UI, including unsubscribe tokens for each listed subscription.

All REST endpoints above require the `X-API-Key` header.

Example subscription request:

```bash
curl -X POST http://localhost:8080/api/subscribe \
  -H 'X-API-Key: dev-api-key' \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","repo":"microsoft/vscode"}'
```

Example see subscriptions request:

```bash
curl -H 'X-API-Key: dev-api-key' \
  "http://localhost:8080/api/subscriptions?email=user@example.com"
```

## gRPC

The service also exposes gRPC with server reflection enabled on `localhost:9090`.
All gRPC methods require metadata header `x-api-key`.

Example with `grpcurl`:

```bash
grpcurl -plaintext \
  -H 'x-api-key: dev-api-key' \
  -d '{"email":"user@example.com","repo":"microsoft/vscode"}' \
  localhost:9090 releasesapi.v1.SubscriptionService/Subscribe
```

List subscriptions with gRPC:

```bash
grpcurl -plaintext \
  -H 'x-api-key: dev-api-key' \
  -d '{"email":"user@example.com"}' \
  localhost:9090 releasesapi.v1.SubscriptionService/GetSubscriptions
```

## Development Notes

- Database migrations run automatically on service startup.
- `last_seen_tag` is seeded from the current latest release during subscription creation so existing releases do not trigger an immediate notification.
- Confirmed subscriptions only are scanned for release notifications.
- GitHub repo existence and latest-release responses are cached in Redis for 10 minutes.
- Prometheus metrics are exposed at `/metrics` with HTTP, GitHub, scanner, and notification counters. A local Prometheus instance is available at `http://localhost:9091` and Grafana with a RED dashboard is at `http://localhost:3000` (Login: `admin/admin`).
- Protected endpoints use API key auth; the default local key is `dev-api-key` unless overridden with `API_KEY`.

## Testing

```bash
docker build --target test -t releases-api-test .
docker run --rm releases-api-test
```
