package service

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
)

var (
	ErrNotFound                    = errors.New("not found")
	ErrAlreadyExists               = errors.New("already exists")
	ErrEmailExists                 = errors.New("email already exists")
	ErrInvalidCredentials          = errors.New("invalid credentials")
	ErrTokenExpired                = errors.New("token expired")
	ErrTokenRevoked                = errors.New("token revoked")
	ErrTokenNotFoundOrRevoked      = errors.New("refresh token not found or already revoked")
	ErrPasswordResetInProgress     = errors.New("password reset in progress")
	ErrTooManyRequests             = errors.New("too many requests")
	ErrInvalidOrExpiredCode        = errors.New("invalid or expired reset code")
	ErrEmailVerificationInProgress = errors.New("email verification in progress")
	ErrEmailAlreadyVerified        = errors.New("email already verified")
)

func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == "23505"
	}

	return false
}
