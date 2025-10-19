package auth

import (
	"api-gateway/internal/dto"
	"context"
	"strings"

	authv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/auth/v1"
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

func (c *Client) Refresh(ctx context.Context, in dto.RefreshRequest) (*dto.RefreshResponse, error) {
	req := &authv1.RefreshRequest{
		RefreshToken: in.RefreshToken,
	}

	resp, err := c.grpc.Refresh(ctx, req)
	if err != nil {
		return nil, err
	}

	out := &dto.RefreshResponse{}
	if t := resp.GetTokens(); t != nil {
		out.Tokens.AccessToken = t.GetAccessToken()
		out.Tokens.RefreshToken = t.GetRefreshToken()
		out.Tokens.AccessExpiresIn = t.GetAccessExpiresIn()
		out.Tokens.RefreshExpiresIn = t.GetRefreshExpiresIn()
	}
	return out, nil
}

func (c *Client) RequestPasswordReset(ctx context.Context, in dto.RequestPasswordResetRequest) error {
	req := &authv1.RequestPasswordResetRequest{
		Email: in.Email,
	}

	_, err := c.grpc.RequestPasswordReset(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) ConfirmPasswordReset(ctx context.Context, in dto.ConfirmPasswordResetRequest) error {
	req := &authv1.ConfirmPasswordResetRequest{
		Code:        in.Code,
		NewPassword: in.NewPassword,
	}

	_, err := c.grpc.ConfirmPasswordReset(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetJwks(ctx context.Context) (*dto.GetJwksResponse, error) {
	req := &authv1.GetJwksRequest{}

	resp, err := c.grpc.GetJwks(ctx, req)
	if err != nil {
		return nil, err
	}

	keys := resp.GetKeys()
	out := &dto.GetJwksResponse{}
	for _, k := range keys {
		jwk := dto.Jwk{
			Kid: k.GetKid(),
			Kty: k.GetKty(),
			Alg: k.GetAlg(),
			Use: k.GetUse(),
			N:   k.GetN(),
			E:   k.GetE(),
		}
		out.Keys = append(out.Keys, jwk)
	}
	return out, nil
}

func (c *Client) Introspect(ctx context.Context, in dto.IntrospectRequest) (*dto.IntrospectResponse, error) {
	req := &authv1.IntrospectRequest{
		AccessToken: in.AccessToken,
	}

	resp, err := c.grpc.Introspect(ctx, req)
	if err != nil {
		return nil, err
	}

	out := &dto.IntrospectResponse{
		Active:  resp.GetActive(),
		UserId:  resp.GetUserId().GetValue(),
		Role:    resp.GetRole().String(),
		ExpUnix: resp.GetExpUnix(),
	}
	if scopes := resp.GetScopes(); len(scopes) > 0 {
		out.Scopes = append(out.Scopes, scopes...)
	}
	return out, nil
}

func (c *Client) Logout(ctx context.Context, in dto.LogoutRequest) error {
	var req *authv1.LogoutRequest
	if rt := strings.TrimSpace(in.RefreshToken); rt != "" {
		req = &authv1.LogoutRequest{Target: &authv1.LogoutRequest_RefreshToken{RefreshToken: rt}}
	} else {
		req = &authv1.LogoutRequest{Target: &authv1.LogoutRequest_All{All: in.All}}
	}
	_, err := c.grpc.Logout(ctx, req)
	if err != nil {
		return err
	}

	return nil
}
