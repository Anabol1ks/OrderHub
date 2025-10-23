package service

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey string

const (
	ctxUserIDKey ctxKey = "userID"
	ctxRoleKey   ctxKey = "role"
)

func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxUserIDKey, id)
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ctxUserIDKey).(uuid.UUID)
	return v, ok
}

type Role string

const (
	RoleCustomer Role = "ROLE_CUSTOMER"
	RoleVendor   Role = "ROLE_VENDOR"
	RoleAdmin    Role = "ROLE_ADMIN"
)

func WithRole(ctx context.Context, r Role) context.Context {
	return context.WithValue(ctx, ctxRoleKey, r)
}
func RoleFromContext(ctx context.Context) (Role, bool) {
	v, ok := ctx.Value(ctxRoleKey).(Role)
	return v, ok
}
