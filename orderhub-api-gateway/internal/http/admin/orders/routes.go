package orders

import (
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type Routes struct {
	h *Handler
}

func Register(rg *gin.RouterGroup, orderConn *grpc.ClientConn, timeout time.Duration) {
	h := NewHandler(orderConn, timeout)
	r := &Routes{h: h}

	rg.GET("/orders", r.h.List)
	rg.GET("/orders/:order_id", r.h.Get)
	rg.POST("/orders/:order_id/cancel", r.h.Cancel)
}
