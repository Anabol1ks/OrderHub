package grpc

import (
	"auth-service/internal/service"
	"context"
	"errors"
	"fmt"
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
	"google.golang.org/protobuf/types/known/emptypb"
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
			AccessToken: tokenPair.AccessToken,
			// возвращаем opaque (не hash)
			RefreshToken:     tokenPair.RefreshOpaque,
			AccessExpiresIn:  tokenPair.AccessExpiresAt.Unix(),
			RefreshExpiresIn: tokenPair.RefreshExpiresAt.Unix(),
		},
	}
	return resp, nil
}

func (s *AuthServer) Logout(ctx context.Context, req *authv1.LogoutRequest) (*emptypb.Empty, error) {
	s.log.Info("Logging out", zap.String("request", fmt.Sprintf("%+v", req)))

	if err := req.Validate(); err != nil {
		s.log.Warn("Invalid refresh request", zap.Error(err))
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	// Особый случай: пользователь мог прислать JSON с двумя полями all=false и refresh_token.
	// В proto это oneof, но разные клиенты могут сформировать неоднозначный payload.
	// Логика: если есть refresh_token (не пустой) — выполняем single logout, игнорируя all=false.
	if rt := req.GetRefreshToken(); strings.TrimSpace(rt) != "" {
		refresh := strings.TrimSpace(rt)
		if err := s.userService.Logout(ctx, refresh); err != nil {
			switch {
			case errors.Is(err, service.ErrTokenNotFoundOrRevoked):
				s.log.Warn("failed", zap.String("op", "Logout"), zap.Error(err))
				return nil, status.Errorf(codes.NotFound, "refresh token not found or revoked: %v", err)
			default:
				s.log.Error("failed", zap.String("op", "Logout"), zap.Error(err))
				return nil, status.Errorf(codes.Internal, "internal server error: %v", err)
			}
		}
		s.log.Info("refresh token revoked")
		return &emptypb.Empty{}, nil
	}

	if req.GetAll() { // mass logout
		cnt, err := s.userService.LogoutAll(ctx)
		if err != nil {
			s.log.Error("failed", zap.String("op", "LogoutAll"), zap.Error(err))
			return nil, status.Errorf(codes.Internal, "internal server error: %v", err)
		}
		s.log.Info("all refresh tokens revoked", zap.Int64("count", cnt))
		return &emptypb.Empty{}, nil
	}

	return nil, status.Error(codes.InvalidArgument, "specify refresh_token or all=true")
}

func (s *AuthServer) GetJwks(ctx context.Context, req *authv1.GetJwksRequest) (*authv1.GetJwksResponse, error) {
	s.log.Info("Getting JWKS", zap.String("request", fmt.Sprintf("%+v", req)))

	keys, err := s.userService.GetJwks(ctx)
	if err != nil {
		s.log.Error("failed", zap.String("op", "GetJwks"), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "internal error")
	}

	resp := &authv1.GetJwksResponse{
		Keys: make([]*authv1.Jwk, 0, len(keys)),
	}
	for _, k := range keys {
		// service.PublicJWK предполагаемые поля: Kid, Kty, Alg, Use, N, E
		resp.Keys = append(resp.Keys, &authv1.Jwk{
			Kid: k.KID,
			Kty: k.Kty,
			Alg: k.Alg,
			Use: k.Use,
			N:   k.N,
			E:   k.E,
		})
	}
	return resp, nil
}

func (s *AuthServer) Introspect(ctx context.Context, req *authv1.IntrospectRequest) (*authv1.IntrospectResponse, error) {
	s.log.Info("Introspecting token", zap.String("request", fmt.Sprintf("%+v", req)))

	active, uid, role, exp, err := s.userService.Introspect(ctx, req.AccessToken)
	if err != nil {
		s.log.Error("failed", zap.String("op", "Introspect"), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "internal error")
	}

	userID := uid
	if !active || userID == uuid.Nil {
		userID = uuid.Nil
	}

	resp := &authv1.IntrospectResponse{
		Active:  active,
		UserId:  toProtoUUID(userID),
		Role:    toProtoRole(role),
		ExpUnix: exp.Unix(),
		Scopes:  nil, // пока без scope
	}
	return resp, nil
}

func (s *AuthServer) RequestPasswordReset(ctx context.Context, req *authv1.RequestPasswordResetRequest) (*emptypb.Empty, error) {
	s.log.Info("Request password reset", zap.String("request", req.Email))

	if err := s.userService.RequestPasswordReset(ctx, req.Email); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			s.log.Warn("failed", zap.String("op", "RequestPasswordReset"), zap.Error(err))
			return nil, status.Errorf(codes.NotFound, "user not found")
		case errors.Is(err, service.ErrPasswordResetInProgress):
			s.log.Warn("failed", zap.String("op", "RequestPasswordReset"), zap.Error(err))
			return nil, status.Errorf(codes.ResourceExhausted, "password reset in progress")
		case errors.Is(err, service.ErrTooManyRequests):
			s.log.Warn("failed", zap.String("op", "RequestPasswordReset"), zap.Error(err))
			return nil, status.Errorf(codes.ResourceExhausted, "too many requests")
		default:
			s.log.Warn("failed", zap.String("op", "RequestPasswordReset"), zap.Error(err))
			return nil, status.Errorf(codes.Internal, "internal server error: %v", err)
		}
	}

	s.log.Info("request password reset send")
	return &emptypb.Empty{}, nil
}

func (s *AuthServer) ConfirmPasswordReset(ctx context.Context, req *authv1.ConfirmPasswordResetRequest) (*emptypb.Empty, error) {
	s.log.Info("Confirm password reset", zap.String("code", req.Code))

	if err := s.userService.ConfirmPasswordReset(ctx, req.Code, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidOrExpiredCode):
			s.log.Warn("failed", zap.String("op", "ConfirmPasswordReset"), zap.Error(err))
			return nil, status.Errorf(codes.InvalidArgument, "invalid or expired code")
		case errors.Is(err, service.ErrNotFound):
			s.log.Warn("failed", zap.String("op", "ConfirmPasswordReset"), zap.Error(err))
			return nil, status.Errorf(codes.NotFound, "user not found")
		default:
			s.log.Warn("failed", zap.String("op", "ConfirmPasswordReset"), zap.Error(err))
			return nil, status.Errorf(codes.Internal, "internal server error: %v", err)
		}
	}

	s.log.Info("reset password confirmed")
	return &emptypb.Empty{}, nil
}

// -------------------------------УТИЛИТЫ----------------------------------

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
