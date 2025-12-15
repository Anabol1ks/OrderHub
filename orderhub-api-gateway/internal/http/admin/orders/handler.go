package orders

import (
	"api-gateway/internal/dto"
	"api-gateway/internal/transport/grpcmeta"
	"context"
	"net/http"
	"strconv"
	"time"

	commonv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/common/v1"
	orderv1 "github.com/Anabol1ks/orderhub-pkg-proto/proto/order/v1"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type Handler struct {
	client  orderv1.OrderServiceClient
	timeout time.Duration
}

func NewHandler(conn *grpc.ClientConn, timeout time.Duration) *Handler {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &Handler{
		client:  orderv1.NewOrderServiceClient(conn),
		timeout: timeout,
	}
}

// ListOrders godoc
// @Summary Получение списка заказов
// @Description Получает список заказов с возможностью фильтрации по пользователю и статусу
// @Security BearerAuth
// @Tags admin/orders
// @Accept json
// @Produce json
// @Param limit query int false "Количество заказов" default(20)
// @Param offset query int false "Смещение" default(0)
// @Param user_id query string false "UUID пользователя"
// @Param status query string false "Статус заказа (PENDING, CONFIRMED, CANCELLED)"
// @Success 200 {object} dto.ListOrdersResponse "Список заказов"
// @Failure 400 {object} dto.ErrorResponse "Неверные параметры"
// @Failure 401 {object} dto.UnauthorizedErrorResponse "Неавторизован"
// @Failure 403 {object} dto.ForbiddenErrorResponse "Доступ запрещён"
// @Failure 500 {object} dto.InternalErrorResponse "Внутренняя ошибка"
// @Router /admin/orders [get]
func (h *Handler) List(c *gin.Context) {
	limit := parseIntDefault(c.Query("limit"), 20)
	offset := parseIntDefault(c.Query("offset"), 0)

	var userID *commonv1.UUID
	if q := c.Query("user_id"); q != "" {
		if _, err := uuid.Parse(q); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
			return
		}
		userID = &commonv1.UUID{Value: q}
	}

	statusEnum := parseOrderStatus(c.Query("status"))

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	ctx = grpcmeta.WithAuthorization(ctx, c.GetHeader("Authorization"))

	resp, err := h.client.ListOrders(ctx, &orderv1.ListOrdersRequest{
		Limit:  int32(limit),
		Offset: int32(offset),
		UserId: userID,
		Status: statusEnum,
	})

	if err != nil {
		writeGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders":      mapOrders(resp.GetOrders()),
		"total":       resp.GetTotal(),
		"next_offset": offset + len(resp.GetOrders()),
	})
}

// GetOrder godoc
// @Summary Получение заказа по ID
// @Description Получает детальную информацию о заказе по его ID
// @Security BearerAuth
// @Tags admin/orders
// @Accept json
// @Produce json
// @Param order_id path string true "UUID заказа"
// @Success 200 {object} dto.GetOrderResponse "Информация о заказе"
// @Failure 400 {object} dto.ErrorResponse "Неверный UUID"
// @Failure 401 {object} dto.UnauthorizedErrorResponse "Неавторизован"
// @Failure 403 {object} dto.ForbiddenErrorResponse "Доступ запрещён"
// @Failure 404 {object} dto.NotFoundErrorResponse "Заказ не найден"
// @Failure 500 {object} dto.InternalErrorResponse "Внутренняя ошибка"
// @Router /admin/orders/{order_id} [get]
func (h *Handler) Get(c *gin.Context) {
	orderID, ok := parseUUIDParam(c, "order_id")
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	ctx = grpcmeta.WithAuthorization(ctx, c.GetHeader("Authorization"))

	resp, err := h.client.GetOrder(ctx, &orderv1.GetOrderRequest{
		OrderId: &commonv1.UUID{Value: orderID},
	})
	if err != nil {
		writeGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order": mapOrder(resp.GetOrder()),
	})
}

// CancelOrder godoc
// @Summary Отмена заказа
// @Description Отменяет заказ с указанием причины
// @Security BearerAuth
// @Tags admin/orders
// @Accept json
// @Produce json
// @Param order_id path string true "UUID заказа"
// @Param cancel body dto.CancelOrderRequest false "Причина отмены"
// @Success 200 {object} dto.CancelOrderResponse "Отменённый заказ"
// @Failure 400 {object} dto.ErrorResponse "Неверный UUID"
// @Failure 401 {object} dto.UnauthorizedErrorResponse "Неавторизован"
// @Failure 403 {object} dto.ForbiddenErrorResponse "Доступ запрещён"
// @Failure 404 {object} dto.NotFoundErrorResponse "Заказ не найден"
// @Failure 500 {object} dto.InternalErrorResponse "Внутренняя ошибка"
// @Router /admin/orders/{order_id}/cancel [post]
func (h *Handler) Cancel(c *gin.Context) {
	orderID, ok := parseUUIDParam(c, "order_id")
	if !ok {
		return
	}
	var body dto.CancelOrderRequest
	_ = c.ShouldBindJSON(&body) // reason может быть пустым

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	ctx = grpcmeta.WithAuthorization(ctx, c.GetHeader("Authorization"))

	resp, err := h.client.CancelOrder(ctx, &orderv1.CancelOrderRequest{
		OrderId: &commonv1.UUID{Value: orderID},
		Reason:  body.Reason,
	})
	if err != nil {
		writeGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order": mapOrder(resp.GetOrder()),
	})
}

func parseUUIDParam(c *gin.Context, name string) (string, bool) {
	v := c.Param(name)
	if _, err := uuid.Parse(v); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return "", false
	}
	return v, true
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return i
}

func parseOrderStatus(s string) commonv1.OrderStatus {
	// если пусто — UNSPECIFIED (без фильтра)
	if s == "" {
		return commonv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
	switch s {
	case "PENDING":
		return commonv1.OrderStatus_ORDER_STATUS_PENDING
	case "CONFIRMED":
		return commonv1.OrderStatus_ORDER_STATUS_CONFIRMED
	case "CANCELLED":
		return commonv1.OrderStatus_ORDER_STATUS_CANCELLED
	}
	// можно поддержать числовой ввод (1/2/3)
	if n, err := strconv.Atoi(s); err == nil {
		return commonv1.OrderStatus(n)
	}
	return commonv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
}

func writeGRPCError(c *gin.Context, err error) {
	st, ok := status.FromError(err)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unknown error"})
		return
	}
	// минимальный mapping gRPC->HTTP
	switch st.Code() {
	case 3: // InvalidArgument
		c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
	case 5: // NotFound
		c.JSON(http.StatusNotFound, gin.H{"error": st.Message()})
	case 7: // PermissionDenied
		c.JSON(http.StatusForbidden, gin.H{"error": st.Message()})
	case 16: // Unauthenticated
		c.JSON(http.StatusUnauthorized, gin.H{"error": st.Message()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": st.Message()})
	}
}

func mapOrders(in []*orderv1.Order) []any {
	out := make([]any, 0, len(in))
	for _, o := range in {
		out = append(out, mapOrder(o))
	}
	return out
}

func mapOrder(o *orderv1.Order) any {
	if o == nil {
		return nil
	}
	items := make([]any, 0, len(o.GetItems()))
	for _, it := range o.GetItems() {
		items = append(items, gin.H{
			"product_id":       it.GetProductId().GetValue(),
			"quantity":         it.GetQuantity(),
			"unit_price_cents": it.GetUnitPriceCents(),
			"line_total_cents": it.GetLineTotalCents(),
			"currency_code":    it.GetCurrencyCode(),
		})
	}
	return gin.H{
		"id":                o.GetId().GetValue(),
		"user_id":           o.GetUserId().GetValue(),
		"status":            o.GetStatus().String(),
		"items":             items,
		"total_price_cents": o.GetTotalPriceCents(),
		"currency_code":     o.GetCurrencyCode(),
		"cancel_reason":     o.GetCancelReason(),
		"created_at":        tsToRFC3339(o.GetCreatedAt()),
		"updated_at":        tsToRFC3339(o.GetUpdatedAt()),
	}
}

func tsToRFC3339(ts interface{ AsTime() time.Time }) string {
	t := ts.AsTime()
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
