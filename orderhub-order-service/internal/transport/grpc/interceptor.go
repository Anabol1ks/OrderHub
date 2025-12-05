package grpc

import (
	"context"
	"strings"

	"order-service/internal/service"

	authv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/auth/v1"
	commonv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/common/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthClient is a minimal interface of AuthService gRPC client we need for introspection.
type AuthClient interface {
	Introspect(ctx context.Context, in *authv1.IntrospectRequest, opts ...grpc.CallOption) (*authv1.IntrospectResponse, error)
}

// NewAuthUnaryServerInterceptor returns a unary interceptor that:
// - allows public methods (health)
// - extracts Bearer token from metadata Authorization
// - calls AuthService.Introspect to validate token
// - injects user id and role into context for downstream handlers
func NewAuthUnaryServerInterceptor(client AuthClient) grpc.UnaryServerInterceptor {
	public := map[string]struct{}{
		"/grpc.health.v1.Health/Check":                                   {},
		"/grpc.health.v1.Health/Watch":                                   {},
		"/grpc.health.v1.Health/List":                                    {},
		"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo": {},
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := public[info.FullMethod]; ok {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata (method=%s)", info.FullMethod)
		}
		authz := getFirst(md, "authorization")
		if authz == "" {
			return nil, status.Errorf(codes.Unauthenticated, "authorization header not found (method=%s)", info.FullMethod)
		}
		prefix := "bearer "
		if len(authz) < len(prefix) || !strings.EqualFold(authz[:len(prefix)], prefix) {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization scheme")
		}
		access := strings.TrimSpace(authz[len(prefix):])
		if access == "" {
			return nil, status.Error(codes.Unauthenticated, "empty bearer token")
		}

		// Validate via Auth service
		resp, err := client.Introspect(ctx, &authv1.IntrospectRequest{AccessToken: access})
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "introspection failed: %v", err)
		}
		if resp == nil || !resp.GetActive() || resp.GetUserId() == nil || resp.GetUserId().GetValue() == "" {
			return nil, status.Error(codes.Unauthenticated, "invalid or inactive token")
		}
		uid, err := uuid.Parse(resp.GetUserId().GetValue())
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid user id")
		}

		// Inject identity
		ctx = service.WithUserID(ctx, uid)
		if role := resp.GetRole(); role != commonv1.Role_ROLE_UNSPECIFIED {
			ctx = service.WithRole(ctx, service.Role(role.String()))
		}
		return handler(ctx, req)
	}
}

func getFirst(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) > 0 {
		return vals[0]
	}
	if key == "authorization" {
		vals = md.Get("Authorization")
		if len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}
