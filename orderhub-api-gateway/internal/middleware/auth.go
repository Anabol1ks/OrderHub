package middleware

import (
	"api-gateway/internal/auth"
	"api-gateway/internal/dto"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Context keys for user info
const (
	CtxUserID   = "user_id"
	CtxUserRole = "user_role"
)

// AuthRequired validates Bearer token using auth service Introspect and injects user info into context.
func AuthRequired(authClient *auth.Client, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		authz := c.GetHeader("Authorization")
		if authz == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewUnauthorizedError("missing Authorization header"))
			return
		}
		token, ok := ExtractBearerToken(authz)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewUnauthorizedError("invalid Authorization header"))
			return
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewUnauthorizedError("empty token"))
			return
		}

		resp, err := authClient.Introspect(c.Request.Context(), dto.IntrospectRequest{AccessToken: token})
		if err != nil || !resp.Active {
			if err != nil {
				log.Warn("introspect failed", zap.Error(err))
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.NewUnauthorizedError("invalid token"))
			return
		}

		// put user info into Gin context
		c.Set(CtxUserID, resp.UserId)
		c.Set(CtxUserRole, resp.Role)
		c.Next()
	}
}

// ExtractBearerToken извлекает токен из заголовка Authorization, устойчиво к лишним символам
// Примеры допустимых значений:
// - "Bearer abc.def.ghi"
// - "Bearer \"abc.def.ghi\""
// - "Bearer abc.def.ghi, extra"
func ExtractBearerToken(authz string) (string, bool) {
	if authz == "" {
		return "", false
	}
	parts := strings.SplitN(authz, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	t := strings.TrimSpace(parts[1])
	// Первичная очистка: снять кавычки по краям, если они есть
	t = strings.Trim(t, " \"'")
	// Обрезать всё после первой запятой (если случайно прилепили JSON)
	if i := strings.IndexRune(t, ','); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	// Доп. очистка: снова снять возможные кавычки после обрезки по запятой
	t = strings.Trim(t, " \"'")
	// На всякий случай, взять первый токен до пробела
	if i := strings.IndexByte(t, ' '); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	// И финальная очистка от кавычек
	t = strings.Trim(t, " \"'")
	return t, true
}
