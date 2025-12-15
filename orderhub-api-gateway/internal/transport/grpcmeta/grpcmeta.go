package grpcmeta

import (
	"context"

	"google.golang.org/grpc/metadata"
)

func WithAuthorization(ctx context.Context, authHeader string) context.Context {
	if authHeader == "" {
		return ctx
	}
	md := metadata.Pairs("authorization", authHeader)
	return metadata.NewOutgoingContext(ctx, md)
}
