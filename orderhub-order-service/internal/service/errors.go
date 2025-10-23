package service

import "errors"

var (
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrOrderNotFound    = errors.New("order not found")
	ErrEmptyItems       = errors.New("empty items")
	ErrQuantityInvalid  = errors.New("quantity must be > 0")
	ErrCurrencyMismatch = errors.New("currency mismatch")
	ErrAlreadyCancelled = errors.New("order already cancelled")
	ErrAlreadyConfirmed = errors.New("order already confirmed")
)
