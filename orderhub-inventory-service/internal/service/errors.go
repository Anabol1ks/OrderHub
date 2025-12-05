package service

import "errors"

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")

	ErrProductNotFound                     = errors.New("product not found")
	ErrInventoryNotFound                   = errors.New("inventory not found")
	ErrReservationEmpty                    = errors.New("reservation items empty")
	ErrReservationExists                   = errors.New("reservation already exists for this order")
	ErrCannotDeleteProductWithReservations = errors.New("cannot delete product with reserved stock")

	ErrSKUAlreadyExists = errors.New("sku already exists for vendor")
	ErrInactiveProduct  = errors.New("product is inactive")
	ErrCurrencyNotRUB   = errors.New("currency must be RUB")
	ErrInvalidQuantity  = errors.New("quantity must be > 0")

	ErrOutOfStock = errors.New("out of stock")
)
