# ADR 0002: Asynchronous Scanner Design

**Status:** Accepted

**Date:** 2026-05-10

## Context
Checking for releases requires polling GitHub for multiple repositories. A synchronous approach would be slow, block the API, and likely hit GitHub rate limits. We need a background mechanism to perform this work.

## Decision
We implemented a background worker loop (`internal/service/scanner.go`) that runs independently of the main API server.
- **Caching:** We use Redis to cache GitHub API responses (repository existence and latest release tags) with a 10-minute TTL to mitigate rate-limiting.
- **Source of Truth:** The database (PostgreSQL) remains the permanent Source of Truth for the `last_seen_tag`.

## Mechanism
1. The scanner polls the database to retrieve all confirmed subscriptions.
2. It queries the GitHub API for the latest release of the target repository.
3. The GitHub client checks the Redis cache first. If the data is missing or expired, it fetches it from GitHub and updates the cache.
4. The scanner compares the "Actual Latest Tag" (from GitHub/Redis) against the "Last Seen Tag" (persisted in PostgreSQL).
5. If the tags differ, a notification is triggered, and the database is updated to reflect the new `last_seen_tag`.

## Consequences
- **Positive:** API response times remain fast as polling is decoupled from request-response cycles.
- **Positive:** Redis cache expiry does not lead to data loss; it only triggers a fresh call to the GitHub API.
- **Positive:** PostgreSQL persistence ensures that subscription progress is maintained across service restarts or cache evictions.
- **Negative:** Increased complexity in managing synchronization between the background worker and the API service.
