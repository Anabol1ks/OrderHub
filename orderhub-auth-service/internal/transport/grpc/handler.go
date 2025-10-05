package grpc

import (
	"auth-service/internal/service"
	"context"
	"errors"
	"net"
	"net/http"
	authv1 "orderhub-proto/auth/v1"
	commonv1 "orderhub-proto/common/v1"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AuthServer struct {
	authv1.UnimplementedAuthServiceServer
	userService *service.AuthService
	log         *zap.Logger
}

func NewAuthServer(userService *service.AuthService, log *zap.Logger) *AuthServer {
	return &AuthServer{
		userService: userService,
		log:         log,
	}
}

func (s *AuthServer) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	s.log.Info("Registering user", zap.String("email", req.Email))
	if err := req.Validate(); err != nil {
		s.log.Warn("Invalid registration request", zap.Error(err))
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}
	u, err := s.userService.Register(ctx, req.Email, req.Password, "ROLE_CUSTOMER")
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAlreadyExists):
			s.log.Warn("failed", zap.String("op", "Register"), zap.Error(err))
			return nil, status.Errorf(codes.AlreadyExists, "user already exists: %v", err)
		default:
			s.log.Error("failed", zap.String("op", "Register"), zap.Error(err))
			return nil, status.Errorf(codes.Internal, "internal server error: %v", err)
		}
	}

	return &authv1.RegisterResponse{
		UserId:    toProtoUUID(u.ID),
		Email:     req.Email,
		Role:      toProtoRole("CUSTOMER"),
		CreatedAt: toProtoTimestamp(),
	}, nil
}

func toProtoUUID(id uuid.UUID) *commonv1.UUID { /* ... */ return &commonv1.UUID{Value: id.String()} }
func toProtoTimestamp() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}
func toProtoRole(role string) commonv1.Role {
	r := strings.TrimSpace(role)
	if r == "" {
		return commonv1.Role_ROLE_CUSTOMER
	}
	rUpper := strings.ToUpper(r)

	// Если пришло "ADMIN" / "CUSTOMER" / "VENDOR" — пробуем добавить префикс
	if !strings.HasPrefix(rUpper, "ROLE_") {
		if v, ok := commonv1.Role_value["ROLE_"+rUpper]; ok {
			return commonv1.Role(v)
		}
	}

	if v, ok := commonv1.Role_value[rUpper]; ok {
		return commonv1.Role(v)
	}

	if v, ok := commonv1.Role_value[r]; ok {
		return commonv1.Role(v)
	}

	return commonv1.Role_ROLE_CUSTOMER
}
func (s *AuthServer) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	s.log.Info("Logging in user", zap.String("email", req.Email))
	if err := req.Validate(); err != nil {
		s.log.Warn("Invalid login request", zap.Error(err))
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	ip := clientIPFromContext(ctx)
	ua := userAgentFromContext(ctx)
	clientID := clientIDFromContextOrGenerate(ctx)

	meta := service.ClientMeta{
		ClientID:  ptrNonEmpty(clientID),
		IP:        ptrNonEmpty(ip),
		UserAgent: ptrNonEmpty(ua),
	}

	grpc.SetHeader(ctx, metadata.Pairs("x-client-id", clientID))
	cookie := (&http.Cookie{
		Name:     "cid",
		Value:    clientID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().AddDate(5, 0, 0),
	}).String()
	grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", cookie))

	u, role, tokenPair, err := s.userService.Login(ctx, req.Email, req.Password, meta)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			s.log.Warn("failed", zap.String("op", "Login"), zap.Error(err))
			return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
		case errors.Is(err, service.ErrInvalidCredentials):
			s.log.Warn("failed", zap.String("op", "Login"), zap.Error(err))
			return nil, status.Errorf(codes.Unauthenticated, "invalid credentials: %v", err)
		default:
			s.log.Error("failed", zap.String("op", "Login"), zap.Error(err))
			return nil, status.Errorf(codes.Internal, "internal server error: %v", err)
		}
	}

	resp := &authv1.LoginResponse{
		UserId: toProtoUUID(u),
		Role:   toProtoRole(role),
		Tokens: &authv1.TokenPair{
			AccessToken:      tokenPair.AccessToken,
			RefreshToken:     tokenPair.RefreshOpaque,
			AccessExpiresIn:  tokenPair.AccessExpiresAt.Unix(),
			RefreshExpiresIn: tokenPair.RefreshExpiresAt.Unix(),
		},
	}
	return resp, nil
}

func (s *AuthServer) Refresh(ctx context.Context, req *authv1.RefreshRequest) (*authv1.RefreshResponse, error) {
	s.log.Info("Refreshing tokens", zap.String("refresh_token", req.RefreshToken))
	if err := req.Validate(); err != nil {
		s.log.Warn("Invalid refresh request", zap.Error(err))
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	ip := clientIPFromContext(ctx)
	ua := userAgentFromContext(ctx)
	clientID := clientIDFromContextOrGenerate(ctx)

	meta := service.ClientMeta{
		ClientID:  ptrNonEmpty(clientID),
		IP:        ptrNonEmpty(ip),
		UserAgent: ptrNonEmpty(ua),
	}

	grpc.SetHeader(ctx, metadata.Pairs("x-client-id", clientID))
	cookie := (&http.Cookie{
		Name:     "cid",
		Value:    clientID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().AddDate(5, 0, 0),
	}).String()
	grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", cookie))

	tokenPair, err := s.userService.Refresh(ctx, req.RefreshToken, meta)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			s.log.Warn("failed", zap.String("op", "Refresh"), zap.Error(err))
			return nil, status.Errorf(codes.NotFound, "refresh token not found: %v", err)
		case errors.Is(err, service.ErrTokenExpired):
			s.log.Warn("failed", zap.String("op", "Refresh"), zap.Error(err))
			return nil, status.Errorf(codes.Unauthenticated, "refresh token expired: %v", err)
		default:
			s.log.Error("failed", zap.String("op", "Refresh"), zap.Error(err))
			return nil, status.Errorf(codes.Internal, "internal server error: %v", err)
		}
	}

	resp := &authv1.RefreshResponse{
		Tokens: &authv1.TokenPair{
			AccessToken:      tokenPair.AccessToken,
			RefreshToken:     tokenPair.RefreshHash,
			AccessExpiresIn:  tokenPair.AccessExpiresAt.Unix(),
			RefreshExpiresIn: tokenPair.RefreshExpiresAt.Unix(),
		},
	}
	return resp, nil
}

func clientIPFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// x-forwarded-for может быть "ip1, ip2, ..."
		if vals := md.Get("x-forwarded-for"); len(vals) > 0 {
			ip := strings.TrimSpace(strings.Split(vals[0], ",")[0])
			if ip != "" {
				return ip
			}
		}
	}
	// fallback: peer
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		host, _, err := net.SplitHostPort(p.Addr.String())
		if err == nil && host != "" {
			return host
		}
		return p.Addr.String()
	}
	return ""
}

func userAgentFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// grpc-gateway пишет оба
		if vals := md.Get("grpcgateway-user-agent"); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
		if vals := md.Get("user-agent"); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}
	return ""
}

func clientIDFromContextOrGenerate(ctx context.Context) string {
	// 1) отдельный заголовок от клиентов / gateway
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-client-id"); len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
			return strings.TrimSpace(vals[0])
		}
		// 2) попробовать достать из Cookie: cid=...
		if vals := md.Get("cookie"); len(vals) > 0 {
			if id := parseClientIDFromCookie(vals[0]); id != "" {
				return id
			}
		}
	}
	// 3) сгенерить новый
	return uuid.NewString()
}

func parseClientIDFromCookie(cookieHeader string) string {
	parts := strings.Split(cookieHeader, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(strings.ToLower(p), "cid=") {
			return strings.TrimPrefix(p, "cid=")
		}
	}
	return ""
}

func ptrNonEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	x := s
	return &x
}
