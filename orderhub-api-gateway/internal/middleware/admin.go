package middleware

import (
	"context"
	"time"

	authv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/auth/v1"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthClient interface {
	Introspect(ctx context.Context, in *authv1.IntrospectRequest, opts ...any) (*authv1.IntrospectResponse, error)
}

type AdminMiddleware struct {
	auth    authv1.AuthServiceClient
	timeout time.Duration
}

func NewAdminMiddleware(authClient authv1.AuthServiceClient, timeout time.Duration) *AdminMiddleware {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &AdminMiddleware{
		auth:    authClient,
		timeout: timeout,
	}
}

func (m *AdminMiddleware) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := ExtractBearerToken(c.GetHeader("Authorization"))
		if !ok {
			c.AbortWithStatusJSON(401, gin.H{"error": "missing or invalid Authorization header"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), m.timeout)
		defer cancel()
		resp, err := m.auth.Introspect(ctx, &authv1.IntrospectRequest{AccessToken: token})
		if err != nil {
			st, _ := status.FromError(err)
			if st != nil && st.Code() == codes.Unauthenticated {
				c.AbortWithStatusJSON(401, gin.H{"error": "unauthenticated"})
				return
			}
			c.AbortWithStatusJSON(401, gin.H{"error": "failed to introspect token"})
			return
		}

		if !resp.GetActive() {
			c.AbortWithStatusJSON(401, gin.H{"error": "token inactive"})
			return
		}

		if resp.GetRole().String() != "ROLE_ADMIN" {
			c.AbortWithStatusJSON(403, gin.H{"error": "forbidden"})
			return
		}

		c.Set("user_id", resp.GetUserId().GetValue())
		c.Set("role", resp.GetRole().String())

		c.Next()
	}
}
