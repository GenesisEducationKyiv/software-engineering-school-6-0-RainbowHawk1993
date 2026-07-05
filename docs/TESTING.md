# Testing Guide

This project uses three types of tests: Unit, Integration, and End-to-End (E2E). All tests are containerized for consistency and ease of use.

## Unit Tests

Unit tests are fast and test individual components in isolation.

### Running Unit Tests (Local)
If you have Go installed:
```bash
make test
```

### Running Unit Tests (Docker)
```bash
docker build --target test -t releases-api-test .
docker run --rm releases-api-test
```

---

## Integration Tests

Integration tests verify the interaction between the application code and external dependencies (PostgreSQL and Redis). They use `httptest.NewServer` and mocked external services (GitHub, Mailer) to ensure deterministic results.

### Running Integration Tests
Integration tests require a running database and Redis. The project provides Makefile targets that use Docker Compose to manage this automatically.

```bash
# Run tests and clean up afterwards
make integration-test-clean

# Run tests but keep containers for debugging
make integration-test-debug
```

Under the hood, this uses `docker-compose.integration.yml` to execute tests in `tests/integration`.

---

## End-to-End (E2E) Tests

E2E tests use Playwright to test the full application flow through the web browser. They run against a fully operational instance of the application including all its dependencies.

### Running E2E Tests
E2E tests run in a dedicated Playwright container.

```bash
# Run E2E tests and clean up afterwards
make e2e-test-clean

# Run E2E tests but keep containers for debugging
make e2e-test-debug
```

Under the hood, this uses `docker-compose.e2e.yml` to execute tests in `tests/e2e` using the Go Playwright bindings.


### Test Artifacts
If an E2E test fails, Playwright generates screenshots and traces. To view these when running in debug mode, you can map the `test-results` volume or use the provided HTML reporter.

## Architecture Tests

Architectural dependency rules (layer boundaries, forbidden imports) are verified by static analysis tests in `tests/architecture/`. They parse Go imports under `internal/` and `cmd/` and fail when a package violates the rules documented in [architecture.md](architecture.md#dependency-rules).

```bash
go test ./tests/architecture/...
```

These run as part of `make test` / `go test ./...`.

---

## Summary of Makefile Targets

| Target | Description |
|--------|-------------|
| `make test` | Run all unit tests |
| `make integration-test-clean` | Build and run integration tests (self-contained) |
| `make e2e-test-clean` | Build and run E2E tests (self-contained) |
| `make clean` | Stop and remove all test and application containers |
