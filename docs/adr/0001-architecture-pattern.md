# ADR 0001: Adoption of Clean Architecture

**Status:** Accepted

**Date:** 2026-05-07

## Context
The project requires high maintainability, testability, and a clear separation between infrastructure (database, API, SMTP) and domain logic.

## Decision
We utilize a layered approach:
- `/internal/api`: Transport layer.
- `/internal/service`: Orchestration and business rules.
- `/internal/store`: Data access layer.
- `/internal/model`: Shared domain entities.

## Consequences
- **Positive:** Domain logic is decoupled from frameworks, making it easier to mock for testing.
- **Neutral:** Increases the number of files due to interfaces and struct wrappers.
