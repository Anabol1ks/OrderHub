package repository

import "gorm.io/gorm"

type Repository struct {
	DB           *gorm.DB
	Products     ProductRepo
	Inventories  InventoryRepo
	Reservations ReservationRepo
}

func buildRepository(db *gorm.DB) *Repository {
	return &Repository{
		DB:           db,
		Products:     NewProductRepo(db),
		Inventories:  NewInventoryRepo(db),
		Reservations: NewReservationRepo(db),
	}
}

func New(db *gorm.DB) *Repository { return buildRepository(db) }

// Глобальная транзакция на весь набор репо
func (r *Repository) WithTx(fn func(tx *Repository) error) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		ctxRepo := &Repository{
			DB:           tx,
			Products:     NewProductRepo(tx),
			Inventories:  NewInventoryRepo(tx),
			Reservations: NewReservationRepo(tx),
		}
		return fn(ctxRepo)
	})
}
