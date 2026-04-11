# GitHub Release Notification API

Single-service Go API for email subscriptions to GitHub repository releases. The service validates repositories through the GitHub API, stores subscriptions in PostgreSQL, sends confirmation and release emails through SMTP, and scans for new releases on a fixed interval.

## Stack

- Go with `chi`
- gRPC alongside REST
- PostgreSQL
- Docker / Docker Compose
- Mailpit for local SMTP capture

## Run Locally

```bash
docker compose up --build
```

Services:

- API: `http://localhost:8080`
- gRPC: `localhost:9090`
- Mailpit UI: `http://localhost:8025`
- PostgreSQL: `localhost:5435`

The project serves a small HTML UI at `http://localhost:8080` for creating, viewing, and unsubscribing subscriptions in the browser.

## Environment

Copy `.env.example` to `.env` if you need custom settings. Important variables:

- `DATABASE_URL`
- `GRPC_PORT`
- `APP_BASE_URL`
- `GITHUB_TOKEN`
- `SCAN_INTERVAL`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_FROM`

## API

- `POST /api/subscribe`
- `GET /api/confirm/{token}`
- `GET /api/unsubscribe/{token}`
- `GET /api/subscriptions?email={email}`

Example subscription request:

```bash
curl -X POST http://localhost:8080/api/subscribe \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","repo":"microsoft/vscode"}'
```

Example see subscriptions request:

```bash
curl "http://localhost:8080/api/subscriptions?email=user@example.com"
```

## gRPC

The service also exposes gRPC with server reflection enabled on `localhost:9090`.

Example with `grpcurl`:

```bash
grpcurl -plaintext \
  -d '{"email":"user@example.com","repo":"microsoft/vscode"}' \
  localhost:9090 releasesapi.v1.SubscriptionService/Subscribe
```

List subscriptions with gRPC:

```bash
grpcurl -plaintext \
  -d '{"email":"user@example.com"}' \
  localhost:9090 releasesapi.v1.SubscriptionService/GetSubscriptions
```

## Development Notes

- Database migrations run automatically on service startup.
- `last_seen_tag` is seeded from the current latest release during subscription creation so existing releases do not trigger an immediate notification.
- Confirmed subscriptions only are scanned for release notifications.

## Testing

```bash
docker build --target test -t releases-api-test .
docker run --rm releases-api-test
```
