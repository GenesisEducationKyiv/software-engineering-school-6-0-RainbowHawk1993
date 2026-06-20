# ADR 0006: Orchestrated Saga for Release Notifications

**Status:** Accepted

**Date:** 2026-06-20

## Context

In [ADR 0005](0005-nats-message-broker-and-notification-service.md), we introduced NATS as a message broker to decouple release detection from notification delivery. We accepted an eventual consistency tradeoff with an "at-least-once (mostly)" delivery guarantee: the `last_seen_tag` was updated in the database *before* publishing the event to NATS. 

If the NATS publish failed after the database update, the new tag remained saved, but the notification was lost, violating the atomicity required for this distributed transaction. We needed a mechanism to ensure that the API database update and the Notification publish step either both succeed or both fail cleanly.

## Decision

We decided to implement an **Orchestrated Saga** pattern to manage the distributed transaction between the API service (Subscription DB) and the Notification service (NATS).

1. **Saga Orchestrator:** We introduced a `ReleaseSagaOrchestrator` within the Scanner microservice to coordinate the process.
2. **Execution Steps:**
    - **Step 1:** The Orchestrator requests the API service (via gRPC) to update the `last_seen_tag` to the new tag.
    - **Step 2:** The Orchestrator publishes the `ReleaseDetected` event to the NATS broker.
3. **Compensation:** If publishing to NATS fails (Step 2), the Orchestrator executes a compensating transaction by calling the API service again to revert the `last_seen_tag` back to the previously known tag.

### Synchronous In-Memory Orchestration

Given the simplicity of this two-step transaction, we opted for an in-memory orchestrator instead of a fully persistent Saga state machine. The Scanner process blocks while the Saga executes. If the Scanner crashes exactly after Step 1 and before Step 2, the compensation won't run, leaving a rare edge case of missed notifications. However, for our current scale, the in-memory rollback provides the vast majority of the atomicity benefits without the massive overhead of distributed saga state tracking.

## Alternatives Considered

| Alternative | Why not chosen |
|-------------|----------------|
| **Two-Phase Commit (2PC)** | Microservices (API, NATS) do not support XA transactions or a shared coordinator easily. 2PC introduces heavy locking, reducing system availability. |
| **Outbox Pattern** | We could have the API service write the event to an `outbox` table in PostgreSQL during the tag update transaction, and a separate worker would publish it. While highly robust, it requires adding a new worker component and table, making it more complex than the orchestrated Saga approach for this specific problem. |
| **Choreographed Saga** | Having the API service publish an "UpdatedTag" event which the Notification service listens to. This moves too much domain logic into the API and removes the Scanner's control over the flow. |

## Consequences

**Positive:**
- Dramatically improves data consistency; if NATS is down, the system rolls back and will retry sending the notification on the next scan interval.
- Maintains the decoupled nature of the microservices.
- No new infrastructure required.

**Negative:**
- Introduces slightly more latency during the scanning process due to the potential for an extra gRPC call during compensation.
- The in-memory orchestrator means a process crash during the Saga execution could still result in a partial failure (though the window for this is extremely small).
