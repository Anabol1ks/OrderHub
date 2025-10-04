package grpc

import (
	"auth-service/internal/service"
	"context"
	"errors"
	authv1 "orderhub-proto/auth/v1"
	commonv1 "orderhub-proto/common/v1"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
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
	u, err := s.userService.Register(ctx, req.Email, req.Password, req.Role.String())
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

	roleEnum := commonv1.Role_ROLE_CUSTOMER
	if v, ok := commonv1.Role_value[string(u.Role)]; ok {
		roleEnum = commonv1.Role(v)
	}

	return &authv1.RegisterResponse{
		UserId:    &commonv1.UUID{Value: u.ID.String()},
		Email:     req.Email,
		Role:      roleEnum,
		CreatedAt: timestamppb.New(time.Now()),
	}, nil
}
