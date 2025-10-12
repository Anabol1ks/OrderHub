package service

import "errors"

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
