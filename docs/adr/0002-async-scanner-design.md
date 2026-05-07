# ADR 0002: Asynchronous Scanner Design

**Status:** Accepted

**Date:** 2026-05-07

## Context
Checking for releases requires polling multiple repositories. A synchronous approach would be too slow and would block the main application.

## Decision
We implemented a background worker loop (`internal/service/scanner.go`) using Redis-based cache with a 10-minute TTL that runs independently of the main API server. It polls the database for confirmed subscriptions and updates the `last_seen_tag` status as it detects new releases.

## Consequences
- **Positive:** API responses remain fast; background work is throttled by the scan interval.
- **Positive:** Centralized control over GitHub API rate limits.
- **Negative**: Adds an infrastructure dependency (Redis) which must be managed in the Docker Compose setup.
