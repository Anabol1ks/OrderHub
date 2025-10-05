package grpc

import (
	"context"
	"strings"

	"auth-service/internal/service"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AuthDeps interface {
	ParseAndValidateAccess(ctx context.Context, token string) (*service.Claims, error)
}

func NewAuthUnaryServerInterceptor(tokens AuthDeps) grpc.UnaryServerInterceptor {
	public := map[string]struct{}{
		"/auth.v1.AuthService/Register":                 {},
		"/auth.v1.AuthService/Login":                    {},
		"/auth.v1.AuthService/Refresh":                  {},
		"/auth.v1.AuthService/Logout":                   {}, // revocation by opaque is safe to expose without access token
		"/auth.v1.AuthService/GetJwks":                  {},
		"/auth.v1.AuthService/RequestEmailVerification": {},
		"/auth.v1.AuthService/ConfirmEmailVerification": {},
		"/auth.v1.AuthService/RequestPasswordReset":     {},
		"/auth.v1.AuthService/ConfirmPasswordReset":     {},
		"/auth.v1.AuthService/Introspect":               {}, // если хочешь — оставь публичным
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Публичные методы — пропускаем без проверки
		if _, ok := public[info.FullMethod]; ok {
			return handler(ctx, req)
		}
		// Backward compatibility: если вдруг сгенерированный метод имеет префикс orderhub.auth.v1 (старый пакет), попробуем обрезать
		if strings.HasPrefix(info.FullMethod, "/orderhub.auth.v1.") {
			// Заменяем на актуальный и проверяем
			alt := strings.Replace(info.FullMethod, "/orderhub.auth.v1.", "/auth.v1.", 1)
			if _, ok := public[alt]; ok {
				return handler(ctx, req)
			}
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

		claims, err := tokens.ParseAndValidateAccess(ctx, access)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid access token: %v", err)
		}
		uid := claims.UserID
		if uid == uuid.Nil {
			return nil, status.Error(codes.Unauthenticated, "invalid subject")
		}

		// Положим идентичность в контекст
		ctx = service.WithUserID(ctx, uid)
		// по желанию: ctx = context.WithValue(ctx, ctxRoleKey, claims.Role)

		return handler(ctx, req)
	}
}

func getFirst(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) > 0 {
		return vals[0]
	}
	// иногда клиенты присылают "Authorization" с заглавной буквы
	if key == "authorization" {
		vals = md.Get("Authorization")
		if len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}
