package dto

// OrderItem представляет позицию в заказе
type OrderItem struct {
	ProductID      string `json:"product_id"`
	Quantity       uint32 `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	LineTotalCents int64  `json:"line_total_cents"`
	CurrencyCode   string `json:"currency_code"`
}

// Order представляет заказ
type Order struct {
	ID              string      `json:"id"`
	UserID          string      `json:"user_id"`
	Status          string      `json:"status"`
	Items           []OrderItem `json:"items"`
	TotalPriceCents int64       `json:"total_price_cents"`
	CurrencyCode    string      `json:"currency_code"`
	CancelReason    string      `json:"cancel_reason,omitempty"`
	CreatedAt       string      `json:"created_at"`
	UpdatedAt       string      `json:"updated_at"`
}

// ListOrdersResponse ответ со списком заказов
type ListOrdersResponse struct {
	Orders     []Order `json:"orders"`
	Total      int32   `json:"total"`
	NextOffset int     `json:"next_offset"`
}

// GetOrderResponse ответ с одним заказом
type GetOrderResponse struct {
	Order Order `json:"order"`
}

// CancelOrderRequest запрос на отмену заказа
type CancelOrderRequest struct {
	Reason string `json:"reason"`
}

// CancelOrderResponse ответ на отмену заказа
type CancelOrderResponse struct {
	Order Order `json:"order"`
}
