# ADR 0003: Supporting Dual API Interfaces (REST and gRPC)

**Status:** Accepted

**Date:** 2026-05-07

## Status
Accepted

## Context
The application needs to be accessible to various types of clients. The web-based frontend requires standard JSON/REST interfaces to function, while future-proofing and low-latency programmatic access suggest the need for a modern RPC framework.

## Decision
We decided to expose the application functionality via both REST (using `chi`) and gRPC (using `google.golang.org/grpc`).

## Consequences
- **Positive:**
    - **Developer Experience:** gRPC provides strongly-typed contracts via Protocol Buffers, making it easier for other Go-based services to integrate with our API.
    - **Flexibility:** The web-based UI remains lightweight and simple to develop using standard browser `fetch` calls, while back-end clients benefit from the efficiency of HTTP/2 and Protobuf.
- **Negative:**
    - **Maintenance:** Any change to the service definition requires updating the `.proto` files, regenerating code, and ensuring the REST handlers (which manually map requests) are also kept in sync.
    - **Complexity:** The codebase requires two sets of authentication/interceptor logic (one for `chi` middleware, one for `grpc.UnaryServerInterceptor`).

## Alternatives Considered
- **grpc-gateway:** Instead of manually maintaining separate REST handlers in `internal/api` and gRPC handlers in `internal/grpcapi`, we could have used `grpc-gateway` to reverse-proxy HTTP/JSON into gRPC.
    - *Why we didn't pick it:* At the time of creation, we preferred full control over the REST API design (specifically the `ui/` routes and standard `chi` middleware) without the added build-step complexity of `grpc-gateway`.
    - *Future pivot:* If the maintenance of keeping two interfaces in sync becomes a bottleneck, we will migrate to `grpc-gateway` to unify the implementation.

## Implementation Details
- **REST:** Built using `go-chi` for simple routing and JSON handling.
- **gRPC:** Built using standard `protoc` generation with server reflection enabled to allow tools like `grpcurl` to inspect the service.
- **Auth:** Both interfaces share the same `API_KEY` requirement, implemented via distinct middleware types to ensure consistent security across both protocols.
