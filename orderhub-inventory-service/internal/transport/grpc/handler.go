package grpc

import (
	"context"
	"errors"
	"inventory-service/internal/models"
	"inventory-service/internal/service"

	commonv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/common/v1"
	inventoryv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/inventory/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Handler struct {
	inventoryv1.UnimplementedInventoryServiceServer
	svc service.InventoryService
}

func NewHandler(svc service.InventoryService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) CreateProduct(ctx context.Context, req *inventoryv1.CreateProductRequest) (*inventoryv1.CreateProductResponse, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	uid, ok := service.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	in, err := toProductInput(req.GetProduct(), uid)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "input: %v", err)
	}
	p, err := h.svc.CreateProduct(ctx, in)
	if err != nil {
		return nil, toStatusErr(err)
	}
	return &inventoryv1.CreateProductResponse{Product: toProtoProduct(p)}, nil
}

func (h *Handler) UpdateProduct(ctx context.Context, req *inventoryv1.UpdateProductRequest) (*inventoryv1.UpdateProductResponse, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}

	id, err := fromUUID(req.GetProductId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid product_id: %v", err)
	}

	patch := fromPatch(req.GetPatch())

	p, err := h.svc.UpdateProduct(ctx, id, patch)
	if err != nil {
		return nil, toStatusErr(err)
	}

	return &inventoryv1.UpdateProductResponse{Product: toProtoProduct(p)}, nil
}

func (h *Handler) GetProduct(ctx context.Context, req *inventoryv1.GetProductRequest) (*inventoryv1.GetProductResponse, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}
	id, err := fromUUID(req.GetProductId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid product_id: %v", err)
	}
	p, err := h.svc.GetProduct(ctx, id)
	if err != nil {
		return nil, toStatusErr(err)
	}
	return &inventoryv1.GetProductResponse{Product: toProtoProduct(p)}, nil
}

func (h *Handler) ListProducts(ctx context.Context, req *inventoryv1.ListProductsRequest) (*inventoryv1.ListProductsResponse, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}
	var vid *uuid.UUID
	if u := req.GetVendorId(); u != nil && u.Value != "" {
		id, err := fromUUID(u)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid vendor_id: %v", err)
		}
		vid = &id
	}
	var onlyActive *bool
	if req.OnlyActive {
		t := true
		onlyActive = &t
	}

	limit := int(req.GetLimit())
	offser := int(req.GetOffset())

	list, total, err := h.svc.ListProducts(ctx, service.ProductListFilter{
		VendorID:   vid,
		Query:      req.Query,
		OnlyActive: onlyActive,
		Limit:      limit,
		Offset:     offser,
	})
	if err != nil {
		return nil, toStatusErr(err)
	}
	out := &inventoryv1.ListProductsResponse{
		Products:   make([]*inventoryv1.Product, 0, len(list)),
		Total:      int32(total),
		NextOffset: nextOffset(offser, limit, int(total)),
	}
	for i := range list {
		out.Products = append(out.Products, toProtoProduct(&list[i]))
	}
	return out, nil
}

func (h *Handler) DeleteProduct(ctx context.Context, req *inventoryv1.DeleteProductRequest) (*emptypb.Empty, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}
	id, err := fromUUID(req.GetProductId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid product_id: %v", err)
	}
	_, err = h.svc.DeleteProduct(ctx, id)
	if err != nil {
		return nil, toStatusErr(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *Handler) BatchGetProducts(ctx context.Context, req *inventoryv1.BatchGetProductsRequest) (*inventoryv1.BatchGetProductsResponse, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}
	ids := make([]uuid.UUID, 0, len(req.GetProductIds()))
	for _, pid := range req.GetProductIds() {
		id, err := fromUUID(pid)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid product_id: %v", err)
		}
		ids = append(ids, id)
	}
	list, err := h.svc.BatchGetProducts(ctx, ids)
	if err != nil {
		return nil, toStatusErr(err)
	}
	out := &inventoryv1.BatchGetProductsResponse{
		Products: make([]*inventoryv1.Product, 0, len(list)),
	}
	for i := range list {
		out.Products = append(out.Products, toProtoProduct(&list[i]))
	}
	return out, nil
}

func (h *Handler) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}

	id, err := fromUUID(req.GetProductId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid product_id: %v", err)
	}
	inv, err := h.svc.GetStock(ctx, id)
	if err != nil {
		return nil, toStatusErr(err)
	}
	return &inventoryv1.GetStockResponse{Stock: toProtoStock(inv)}, nil
}

func (h *Handler) SetStock(ctx context.Context, req *inventoryv1.SetStockRequest) (*inventoryv1.SetStockResponse, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}
	id, err := fromUUID(req.GetProductId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid product_id: %v", err)
	}

	inv, err := h.svc.SetStock(ctx, id, req.GetAvailable())
	if err != nil {
		return nil, toStatusErr(err)
	}
	return &inventoryv1.SetStockResponse{Stock: toProtoStock(inv)}, nil
}

func (h *Handler) AdjustStock(ctx context.Context, req *inventoryv1.AdjustStockRequest) (*inventoryv1.AdjustStockResponse, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}
	id, err := fromUUID(req.GetProductId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid product_id: %v", err)
	}
	inv, err := h.svc.AdjustStock(ctx, id, req.GetDelta())
	if err != nil {
		return nil, toStatusErr(err)
	}
	return &inventoryv1.AdjustStockResponse{Stock: toProtoStock(inv)}, nil
}

func (h *Handler) Reserve(ctx context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}

	oid, err := fromUUID(req.GetOrderId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order_id: %v", err)
	}
	items := make([]service.ReserveItem, 0, len(req.GetItems()))
	for _, it := range req.GetItems() {
		pid, err := fromUUID(it.GetProductId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid product_id: %v", err)
		}
		items = append(items, service.ReserveItem{
			ProductID: pid,
			Quantity:  it.GetQuantity(),
		})
	}

	res, err := h.svc.Reserve(ctx, oid, items)
	if err != nil {
		return nil, toStatusErr(err)
	}
	out := &inventoryv1.ReserveResponse{
		OkItems:     make([]*inventoryv1.ReserveOkItem, 0, len(res.OK)),
		FailedItems: make([]*inventoryv1.ReserveFailedItem, 0, len(res.Failed)),
	}
	for _, okIt := range res.OK {
		out.OkItems = append(out.OkItems, &inventoryv1.ReserveOkItem{
			ProductId: toUUID(okIt.ProductID),
			Quantity:  okIt.Quantity,
		})
	}
	for _, f := range res.Failed {
		out.FailedItems = append(out.FailedItems, &inventoryv1.ReserveFailedItem{
			ProductId: toUUID(f.ProductID),
			Requested: f.Requested,
			Reason:    f.Reason,
		})
	}
	return out, nil
}

func (h *Handler) Release(ctx context.Context, req *inventoryv1.ReleaseRequest) (*emptypb.Empty, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}

	oid, err := fromUUID(req.GetOrderId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order_id: %v", err)
	}
	if _, err := h.svc.Release(ctx, oid); err != nil {
		return nil, toStatusErr(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *Handler) Confirm(ctx context.Context, req *inventoryv1.ConfirmRequest) (*emptypb.Empty, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}

	oid, err := fromUUID(req.GetOrderId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order_id: %v", err)
	}
	if _, err := h.svc.Confirm(ctx, oid); err != nil {
		return nil, toStatusErr(err)
	}
	return &emptypb.Empty{}, nil
}

// хелп
func toProductInput(pi *inventoryv1.ProductInput, vendorID uuid.UUID) (service.ProductInput, error) {
	if pi == nil {
		return service.ProductInput{}, errors.New("product is required")
	}
	return service.ProductInput{
		VendorID:     vendorID,
		SKU:          pi.GetSku(),
		Name:         pi.GetName(),
		Description:  pi.GetDescription(),
		PriceCents:   pi.GetPriceCents(),
		CurrencyCode: pi.GetCurrencyCode(),
		IsActive:     pi.GetIsActive(),
	}, nil
}

func fromPatch(p *inventoryv1.ProductPatch) service.ProductPatch {
	if p == nil {
		return service.ProductPatch{}
	}
	var out service.ProductPatch
	if p.Sku != nil {
		v := p.Sku.Value
		out.SKU = &v
	}
	if p.Name != nil {
		v := p.Name.Value
		out.Name = &v
	}
	if p.Description != nil {
		v := p.Description.Value
		out.Description = &v
	}
	if p.PriceCents != nil {
		v := p.PriceCents.Value
		out.PriceCents = &v
	}
	if p.CurrencyCode != nil {
		v := p.CurrencyCode.Value
		out.CurrencyCode = &v
	}
	if p.IsActive != nil {
		v := p.IsActive.Value
		out.IsActive = &v
	}
	return out
}

func toProtoProduct(p *models.Product) *inventoryv1.Product {
	return &inventoryv1.Product{
		Id:           toUUID(p.ID),
		VendorId:     toUUID(p.VendorID),
		Sku:          p.SKU,
		Name:         p.Name,
		Description:  p.Description,
		PriceCents:   p.PriceCents,
		CurrencyCode: p.CurrencyCode,
		IsActive:     p.IsActive,
		CreatedAt:    timestamppb.New(p.CreatedAt),
		UpdatedAt:    timestamppb.New(p.UpdatedAt),
	}
}

func toProtoStock(s *models.Inventory) *inventoryv1.Stock {
	return &inventoryv1.Stock{
		ProductId: toUUID(s.ProductID),
		Available: s.Available,
		Reserved:  s.Reserved,
		UpdatedAt: timestamppb.New(s.UpdatedAt),
	}
}

func toUUID(id uuid.UUID) *commonv1.UUID {
	return &commonv1.UUID{Value: id.String()}
}

func fromUUID(u *commonv1.UUID) (uuid.UUID, error) {
	if u == nil || u.Value == "" {
		return uuid.Nil, errors.New("empty uuid")
	}
	return uuid.Parse(u.Value)
}

func nextOffset(current, limit, total int) int32 {
	if limit <= 0 {
		return -1
	}
	next := current + limit
	if next >= total {
		return -1
	}
	return int32(next)
}

func toStatusErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, service.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, "unauthorized")
	case errors.Is(err, service.ErrForbidden):
		return status.Error(codes.PermissionDenied, "forbidden")
	case errors.Is(err, service.ErrProductNotFound),
		errors.Is(err, service.ErrInventoryNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, service.ErrSKUAlreadyExists),
		errors.Is(err, service.ErrCurrencyNotRUB),
		errors.Is(err, service.ErrInvalidQuantity),
		errors.Is(err, service.ErrReservationEmpty),
		errors.Is(err, service.ErrReservationExists):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrOutOfStock):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal: %v", err)
	}
}
