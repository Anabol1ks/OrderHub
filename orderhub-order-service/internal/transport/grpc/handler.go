package grpc

import (
	"context"
	"errors"
	"order-service/internal/models"
	"order-service/internal/service"

	commonv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/common/v1"
	orderv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/order/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OrderServer struct {
	orderv1.UnimplementedOrderServiceServer
	svc service.OrderService
}

func NewOrderServer(svc service.OrderService) *OrderServer {
	return &OrderServer{
		svc: svc,
	}
}

func (s *OrderServer) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
	}

	in, err := toCreateInput(req)
	if err != nil {
		return nil, toStatusErr(err)
	}
	o, err := s.svc.CreateOrder(ctx, in)
	if err != nil {
		return nil, toStatusErr(err)
	}
	return &orderv1.CreateOrderResponse{Order: toProtoOrder(o)}, nil
}

func (s *OrderServer) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}
	id, err := fromUUID(req.GetOrderId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order_id: %v", err)
	}
	o, err := s.svc.GetOrder(ctx, id)
	if err != nil {
		return nil, toStatusErr(err)
	}
	return &orderv1.GetOrderResponse{Order: toProtoOrder(o)}, nil
}

func (s *OrderServer) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}
	f, err := toListFilter(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "filter: %v", err)
	}
	list, total, err := s.svc.ListOrders(ctx, f)
	if err != nil {
		return nil, toStatusErr(err)
	}
	out := &orderv1.ListOrdersResponse{
		Orders:     make([]*orderv1.Order, 0, len(list)),
		Total:      int32(total),
		NextOffset: nextOffset(int(req.Offset), int(req.Limit), int(total)),
	}
	for i := range list {
		out.Orders = append(out.Orders, toProtoOrder(&list[i]))
	}
	return out, nil
}

func (s *OrderServer) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	if v, ok := any(req).(interface{ ValidateAll() error }); ok {
		if err := v.ValidateAll(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
		}
	}
	id, err := fromUUID(req.GetOrderId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order_id: %v", err)
	}
	var reasonPtr *string
	if r := req.GetReason(); r != "" {
		reasonPtr = &r
	}
	o, err := s.svc.CancelOrder(ctx, id, reasonPtr)
	if err != nil {
		return nil, toStatusErr(err)
	}
	return &orderv1.CancelOrderResponse{Order: toProtoOrder(o)}, nil
}

func toCreateInput(req *orderv1.CreateOrderRequest) (service.CreateOrderInput, error) {
	items := make([]service.CreateOrderItem, 0, len(req.GetItems()))
	for _, it := range req.GetItems() {
		pid, err := fromUUID(it.GetProductId())
		if err != nil {
			return service.CreateOrderInput{}, status.Errorf(codes.InvalidArgument, "invalid product_id: %v", err)
		}
		items = append(items, service.CreateOrderItem{
			ProductID: pid,
			Quantity:  it.GetQuantity(),
		})
	}
	return service.CreateOrderInput{
		Items:   items,
		Comment: req.GetComment(),
	}, nil
}

func toListFilter(ctx context.Context, req *orderv1.ListOrdersRequest) (service.ListFilter, error) {
	var (
		uidPtr *uuid.UUID
		stPtr  *models.OrderStatus
	)

	// статус
	if req.GetStatus() != commonv1.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		s := fromProtoStatus(req.GetStatus())
		stPtr = &s
	}

	// разбор user_id: только админ может указывать чужой user_id
	if u := req.GetUserId(); u != nil && u.Value != "" {
		// проверим роль в контексте
		if role, ok := service.RoleFromContext(ctx); !ok || role != service.RoleAdmin {
			// non-admin не может фильтровать по user_id
			return service.ListFilter{}, errors.New("user_id filter allowed only for admin")
		}
		id, err := fromUUID(u)
		if err != nil {
			return service.ListFilter{}, err
		}
		uidPtr = &id
	} else {
		// если не указан user_id: для не-admin — по умолчанию ограничиваем по токену
		if role, ok := service.RoleFromContext(ctx); ok && role != service.RoleAdmin {
			if uid, ok2 := service.UserIDFromContext(ctx); ok2 {
				uidPtr = &uid
			}
		}
		// для admin uidPtr остаётся nil => просмотр всех пользователей
	}

	limit := int(req.GetLimit())
	offset := int(req.GetOffset())

	return service.ListFilter{
		UserID: uidPtr,
		Status: stPtr,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func toProtoOrder(o *models.Order) *orderv1.Order {
	items := make([]*orderv1.OrderItem, 0, len(o.Items))
	for i := range o.Items {
		it := &o.Items[i]
		items = append(items, &orderv1.OrderItem{
			ProductId:      toUUID(it.ProductID),
			Quantity:       it.Quantity,
			UnitPriceCents: it.UnitPriceCents,
			LineTotalCents: it.LineTotalCents,
			CurrencyCode:   it.CurrencyCode,
		})
	}
	return &orderv1.Order{
		Id:              toUUID(o.ID),
		UserId:          toUUID(o.UserID),
		Status:          toProtoStatus(o.Status),
		Items:           items,
		TotalPriceCents: o.TotalPriceCents,
		CurrencyCode:    o.CurrencyCode,
		CancelReason:    strOrEmpty(o.CancelReason),
		CreatedAt:       timestamppb.New(o.CreatedAt),
		UpdatedAt:       timestamppb.New(o.UpdatedAt),
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

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func toProtoStatus(s models.OrderStatus) commonv1.OrderStatus {
	switch s {
	case models.OrderStatusPending:
		return commonv1.OrderStatus_ORDER_STATUS_PENDING
	case models.OrderStatusConfirmed:
		return commonv1.OrderStatus_ORDER_STATUS_CONFIRMED
	case models.OrderStatusCancelled:
		return commonv1.OrderStatus_ORDER_STATUS_CANCELLED
	default:
		return commonv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func fromProtoStatus(s commonv1.OrderStatus) models.OrderStatus {
	switch s {
	case commonv1.OrderStatus_ORDER_STATUS_PENDING:
		return models.OrderStatusPending
	case commonv1.OrderStatus_ORDER_STATUS_CONFIRMED:
		return models.OrderStatusConfirmed
	case commonv1.OrderStatus_ORDER_STATUS_CANCELLED:
		return models.OrderStatusCancelled
	default:
		return models.OrderStatusPending
	}
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
	case errors.Is(err, service.ErrOrderNotFound):
		return status.Error(codes.NotFound, "order not found")
	case errors.Is(err, service.ErrEmptyItems),
		errors.Is(err, service.ErrQuantityInvalid),
		errors.Is(err, service.ErrCurrencyMismatch):
		return status.Errorf(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrAlreadyCancelled),
		errors.Is(err, service.ErrAlreadyConfirmed):
		return status.Errorf(codes.FailedPrecondition, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal: %v", err)
	}
}
