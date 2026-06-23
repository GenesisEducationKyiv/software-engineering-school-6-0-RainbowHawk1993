**Date:** 2026-05-06

# Database Schema

The system uses a single table `subscriptions` to manage state.

```mermaid
erDiagram
    SUBSCRIPTION {
        bigint id PK
        string email
        string repo_owner
        string repo_name
        bool confirmed
        string confirm_token
        string unsubscribe_token
        string last_seen_tag
        timestamp created_at
        timestamp updated_at
    }
```

## Migration Notes

Database migrations are automatically applied on application startup via internal/migrations.
The confirm_token and unsubscribe_token are 64-character hex strings generated for security.
