# ADR 0005: NATS Message Broker and Notification Service

**Status:** Accepted

**Date:** 2026-06-13

## Context

The Scanner microservice was directly sending emails via SMTP whenever a new GitHub release was detected. This coupled the release detection logic with the notification delivery logic. If the SMTP server was slow or unavailable, the scanner loop would be blocked or drop notifications.

As the system grows, we may want to add new notification channels (e.g., Slack, Discord, Webhooks) or retry notification deliveries independently of the release detection process.

## Decision

We decided to introduce **NATS** as a lightweight message broker and extract the notification logic into a dedicated **Notification Service**.

1. **Scanner modification:** The scanner no longer sends emails. Instead, when it detects a new release and updates the `last_seen_tag` in the API, it publishes a `ReleaseDetected` event to the NATS `releases.detected` subject.
2. **New Notification Service:** A new deployable unit (`cmd/notification`) connects to NATS, subscribes to `releases.detected`, and handles sending the emails via SMTP.

### Event Schema

```go
type ReleaseDetected struct {
	Email            string `json:"email"`
	RepoOwner        string `json:"repo_owner"`
	RepoName         string `json:"repo_name"`
	Tag              string `json:"tag"`
	UnsubscribeToken string `json:"unsubscribe_token"`
}
```

### Delivery Guarantee

We opted for an **"at-least-once (mostly)"** approach:
1. The scanner updates the `last_seen_tag` via gRPC.
2. If successful, the scanner publishes the event to NATS.

*Note on failure modes:* If the NATS publish fails after the tag is updated, the notification is lost. We explicitly accepted this tradeoff to avoid sending duplicate notifications on every scanner restart, which would happen if we published before updating the tag.

## Alternatives Considered

| Alternative | Why not chosen |
|-------------|----------------|
| RabbitMQ | A solid option, but heavier and more complex to configure than NATS. For our current scale, NATS provides a simpler, zero-config solution. |
| Redis Pub/Sub | We already have Redis for caching, but Redis Pub/Sub offers no durability or consumer groups, making it unsuitable for reliable event delivery. |
| Kafka | Severe overkill for our volume of events. |
| Keep direct SMTP | Does not decouple the domains and makes adding new notification channels difficult. |

## Consequences

**Positive:**
- Decouples release detection from notification delivery.
- Allows independent scaling of the Notification Service.
- Easy to add new consumers for `releases.detected` without touching the scanner code.
- Simplifies the Scanner dependencies (no more SMTP configuration).

**Negative:**
- Adds a new infrastructural dependency (NATS).
- Adds a new microservice to deploy and monitor.
- Introduces eventual consistency for notifications.
