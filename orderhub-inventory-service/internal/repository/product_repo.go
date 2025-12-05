package repository

import (
	"context"
	"errors"
	"inventory-service/internal/models"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProductListFilter struct {
	VendorID   *uuid.UUID
	Query      string // по name/sku
	OnlyActive *bool
	Limit      int
	Offset     int
}

type ProductRepo interface {
	Create(ctx context.Context, p *models.Product) error
	UpdateFields(ctx context.Context, id uuid.UUID, fields map[string]any) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Product, error)
	GetByVendorAndSKU(ctx context.Context, vendorID uuid.UUID, sku string) (*models.Product, error)
	List(ctx context.Context, f ProductListFilter) ([]models.Product, int64, error)
	Delete(ctx context.Context, id uuid.UUID) (bool, error)
	BatchGetByIDs(ctx context.Context, ids []uuid.UUID) ([]models.Product, error)
	EnsureInventoryRow(ctx context.Context, productID uuid.UUID) error
}

type productRepo struct {
	db *gorm.DB
}

func NewProductRepo(db *gorm.DB) ProductRepo { return &productRepo{db: db} }

func (r *productRepo) Create(ctx context.Context, p *models.Product) error {
	return r.db.WithContext(ctx).Select("*").Create(p).Error
}

func (r *productRepo) UpdateFields(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&models.Product{}).Where("id = ?", id).Updates(fields).Error
}

func (r *productRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	var p models.Product
	if err := r.db.WithContext(ctx).First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *productRepo) GetByVendorAndSKU(ctx context.Context, vendorID uuid.UUID, sku string) (*models.Product, error) {
	var p models.Product
	err := r.db.WithContext(ctx).Where("vendor_id = ? AND lower(sku) = lower(?)", vendorID, sku).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &p, err
}

func (r *productRepo) List(ctx context.Context, f ProductListFilter) ([]models.Product, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Product{})

	if f.VendorID != nil {
		q = q.Where("vendor_id = ?", *f.VendorID)
	}

	if f.OnlyActive != nil {
		q = q.Where("is_active = ?", *f.OnlyActive)
	}

	if s := strings.TrimSpace(f.Query); s != "" {
		q = q.Where("lower(name) LIKE lower(?) OR lower(sku) LIKE lower(?)", "%"+s+"%", "%"+s+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if f.Limit <= 0 {
		f.Limit = 20
	}

	if f.Offset < 0 {
		f.Offset = 0
	}

	var list []models.Product
	if err := q.Order("created_at DESC").Limit(f.Limit).Offset(f.Offset).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *productRepo) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	tx := r.db.WithContext(ctx).Delete(&models.Product{}, "id = ?", id)
	return tx.RowsAffected > 0, tx.Error
}

func (r *productRepo) BatchGetByIDs(ctx context.Context, ids []uuid.UUID) ([]models.Product, error) {
	if len(ids) == 0 {
		return []models.Product{}, nil
	}

	var list []models.Product
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&list).Error
	return list, err
}

func (r *productRepo) EnsureInventoryRow(ctx context.Context, productID uuid.UUID) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&models.Inventory{ProductID: productID}).Error
}
