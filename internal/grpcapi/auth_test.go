package grpcapi

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestGRPCAPIKeyValid(t *testing.T) {
	t.Parallel()

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-api-key", "secret-key"))
	if !grpcAPIKeyValid(ctx, "secret-key") {
		t.Fatal("expected matching grpc api key to validate")
	}
	if grpcAPIKeyValid(ctx, "wrong-key") {
		t.Fatal("expected mismatched grpc api key to fail")
	}
	if grpcAPIKeyValid(context.Background(), "secret-key") {
		t.Fatal("expected missing grpc api key to fail")
	}
}
