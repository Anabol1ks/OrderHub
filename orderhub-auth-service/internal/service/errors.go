package service

import "errors"

var (
	ErrNotFound                = errors.New("not found")
	ErrAlreadyExists           = errors.New("already exists")
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrTokenExpired            = errors.New("token expired")
	ErrTokenRevoked            = errors.New("token revoked")
	ErrTokenNotFoundOrRevoked  = errors.New("refresh token not found or already revoked")
	ErrPasswordResetInProgress = errors.New("password reset in progress")
	ErrTooManyRequests         = errors.New("too many requests")
)
