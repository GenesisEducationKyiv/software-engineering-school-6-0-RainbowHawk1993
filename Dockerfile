FROM golang:1.24-alpine AS base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM base AS test
RUN go test ./...
CMD ["go", "test", "./..."]

FROM base AS integration-tests
# Don't test during build - docker-compose will run tests at runtime
# when database and redis are available
CMD ["go", "test", "-tags=integration", "-v", "./tests/integration"]

FROM base AS build
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/releases-api ./cmd/server

FROM alpine:3.21 AS runtime
RUN adduser -D -g '' appuser
USER appuser
WORKDIR /app
COPY --from=build /bin/releases-api /app/releases-api
EXPOSE 8080 9090
ENTRYPOINT ["/app/releases-api"]
