package auth

import (
	"api-gateway/internal/dto"
	"context"
	authv1 "orderhub-proto/auth/v1"
)

// Client обёртка над gRPC AuthServiceClient, инкапсулирующая маппинг
// HTTP DTO <-> gRPC proto. Добавлять сюда методы: Register, Login, Refresh и т.д.
type Client struct {
	grpc authv1.AuthServiceClient
}

func NewClient(grpcClient authv1.AuthServiceClient) *Client { return &Client{grpc: grpcClient} }

func (c *Client) Register(ctx context.Context, in dto.RegisterRequest) (*dto.RegisterResponse, error) {
	req := &authv1.RegisterRequest{
		Email:    in.Email,
		Password: in.Password,
	}

	resp, err := c.grpc.Register(ctx, req)
	if err != nil {
		return nil, err
	}

	out := &dto.RegisterResponse{
		UserId:    resp.GetUserId().GetValue(),
		Email:     resp.GetEmail(),
		Role:      resp.GetRole().String(), // enum -> string, например ROLE_CUSTOMER
		CreatedAt: resp.GetCreatedAt().AsTime().Format("2006-01-02T15:04:05Z07:00"),
	}
	return out, nil
}

func (c *Client) Login(ctx context.Context, in dto.LoginRequest) (*dto.LoginResponse, error) {
	req := &authv1.LoginRequest{
		Email:    in.Email,
		Password: in.Password,
	}

	resp, err := c.grpc.Login(ctx, req)
	if err != nil {
		return nil, err
	}

	out := &dto.LoginResponse{
		UserId: resp.GetUserId().GetValue(),
		Role:   resp.GetRole().String(),
	}

	if t := resp.GetTokens(); t != nil {
		out.Tokens.AccessToken = t.GetAccessToken()
		out.Tokens.RefreshToken = t.GetRefreshToken()
		out.Tokens.AccessExpiresIn = t.GetAccessExpiresIn()
		out.Tokens.RefreshExpiresIn = t.GetRefreshExpiresIn()
	}

	return out, nil
}
