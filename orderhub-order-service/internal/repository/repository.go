package repository

import "gorm.io/gorm"

type Repository struct {
	DB         *gorm.DB
	Orders     OrderRepo
	OrderItems OrderItemRepo
}

func buildRepository(db *gorm.DB) *Repository {
	return &Repository{
		DB:         db,
		Orders:     NewOrderRepo(db),
		OrderItems: NewOrderItemRepo(db),
	}
}

func New(db *gorm.DB) *Repository { return buildRepository(db) }
