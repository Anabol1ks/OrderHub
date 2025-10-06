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

// Register выполняет:
// 1. Маппинг входного dto.RegisterRequest -> authv1.RegisterRequest
// 2. Вызов удалённого gRPC метода
// 3. Маппинг ответа authv1.RegisterResponse -> dto.RegisterResponse
// Ошибки из gRPC просто пробрасываются наверх — handler решает что отвечать.
func (c *Client) Register(ctx context.Context, in dto.RegisterRequest) (*dto.RegisterResponse, error) {
	// Шаг 1: dto -> proto
	req := &authv1.RegisterRequest{
		Email:    in.Email,
		Password: in.Password,
	}

	// Шаг 2: вызов gRPC
	resp, err := c.grpc.Register(ctx, req)
	if err != nil {
		return nil, err
	}

	// Шаг 3: proto -> dto
	out := &dto.RegisterResponse{
		UserId:    resp.GetUserId().GetValue(),
		Email:     resp.GetEmail(),
		Role:      resp.GetRole().String(), // enum -> string, например ROLE_CUSTOMER
		CreatedAt: resp.GetCreatedAt().AsTime().Format("2006-01-02T15:04:05Z07:00"),
	}
	return out, nil
}
