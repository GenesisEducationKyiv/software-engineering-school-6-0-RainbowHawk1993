FROM golang:1.24-alpine AS base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM base AS test
RUN go test ./...
CMD ["go", "test", "./..."]

FROM base AS integration-tests
CMD ["go", "test", "-tags=integration", "-v", "./tests/integration"]

FROM base AS build
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/releases-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/releases-scanner ./cmd/scanner

FROM alpine:3.21 AS runtime
RUN adduser -D -g '' appuser
USER appuser
WORKDIR /app
COPY --from=build /bin/releases-api /app/releases-api
COPY --from=build /bin/releases-scanner /app/releases-scanner
EXPOSE 8080 9090 9091
ENTRYPOINT ["/app/releases-api"]

FROM alpine:3.21 AS scanner-runtime
RUN adduser -D -g '' appuser
USER appuser
WORKDIR /app
COPY --from=build /bin/releases-scanner /app/releases-scanner
ENTRYPOINT ["/app/releases-scanner"]
