package grpcapi

import (
	"context"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const apiKeyHeader = "x-api-key"

func UnaryAPIKeyInterceptor(expectedKey string) grpc.UnaryServerInterceptor {
	trimmedKey := strings.TrimSpace(expectedKey)
	if trimmedKey == "" {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !grpcAPIKeyValid(ctx, trimmedKey) {
			return nil, status.Error(codes.Unauthenticated, "missing or invalid api key")
		}

		return handler(ctx, req)
	}
}

func grpcAPIKeyValid(ctx context.Context, expected string) bool {
	if expected == "" {
		return true
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}

	values := md.Get(apiKeyHeader)
	if len(values) == 0 {
		return false
	}

	provided := strings.TrimSpace(values[0])
	if len(provided) != len(expected) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
