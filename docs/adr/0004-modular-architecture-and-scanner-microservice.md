# ADR 0004: Modular Architecture and Scanner Microservice Extraction

**Status:** Accepted

**Date:** 2026-06-07

## Context

The application started as a single Go binary with layered Clean Architecture (ADR 0001). Subscription lifecycle management and the background release scanner lived in the same process (`cmd/server`), sharing PostgreSQL access and infrastructure wiring.

As the domain grew, this layout created coupling:

- Business logic for two distinct bounded contexts was mixed in `internal/service/`
- The scanner ran as a goroutine inside the API process, preventing independent scaling or deployment
- Data access boundaries were implicit rather than enforced between domains

ADR 0002 established the scanner as a background worker, and `docs/architecture.md` noted that horizontal scaling of the scanner may require a distributed job queue. The current step extracts the scanner into its own deployable unit while reorganizing the monolith API into explicit domain modules.

## Decision

### 1. Domain-module structure

Replace the layer-only layout with bounded-context modules:

| Module | Responsibility |
|--------|----------------|
| `subscription` | Subscribe, confirm, unsubscribe, list; owns PostgreSQL data |
| `scanner` | Poll GitHub for new releases, send notifications, update `last_seen_tag` |
| `github` | GitHub API client and Redis cache (shared library) |
| `notification` | Email templates and SMTP delivery (shared library) |
| `platform` | Config, migrations, metrics, domain errors |
| `transport` | REST, public gRPC, internal gRPC adapters |

### 2. Scanner as a separate microservice

- **`cmd/api`**: REST + public gRPC + internal gRPC; owns database migrations and subscription writes
- **`cmd/scanner`**: background worker only; no direct database access

### 3. Inter-service communication

The scanner retrieves subscriptions and updates `last_seen_tag` through an **internal gRPC API** (`InternalSubscriptionService` on port `9091`), not shared PostgreSQL access.

Contract: `proto/releasesapi/v1/subscription_internal.proto`

- `ListConfirmedForScan`
- `UpdateLastSeenTag`

Both public and internal gRPC endpoints require the same `x-api-key` metadata.

### 4. Data ownership

PostgreSQL remains the single source of truth for subscription state. Only the API service connects to the database. The scanner is a stateless worker that delegates persistence to the API via gRPC.

## Alternatives Considered

| Alternative | Why not chosen |
|-------------|----------------|
| Shared PostgreSQL for scanner | Simpler to implement but violates service boundaries and duplicates data-access logic |
| Message queue (Temporal/RabbitMQ) | Valuable at higher scale, but excessive for current subscription volume |
| Keep scanner as goroutine | Does not satisfy the requirement for a separate microservice |
| REST internal API | gRPC reuses existing infrastructure and provides a typed contract |

## Consequences

**Positive:**

- Independent deploy and scale of the scanner worker
- Clear domain boundaries with explicit ports between modules
- API response times remain unaffected by scanner load
- Subscription data access is centralized in one service

**Negative:**

- Additional network hop between scanner and API on every scan cycle
- Two binaries, two Docker images/targets, and more compose services to operate
- Internal proto contract must be maintained alongside the public API

**Neutral:**

- Partially supersedes the directory layout described in ADR 0001; Clean Architecture principles (interfaces at boundaries, testable domain logic) are preserved within each module

## Implementation Notes

- Docker Compose services: `api`, `scanner`, `db`, `redis`, `mailpit`
- Scanner environment: `SUBSCRIPTION_API_GRPC_ADDR` (default `api:9091`)
- API environment: `INTERNAL_GRPC_PORT` (default `9091`)
