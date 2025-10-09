package handlers

import (
	"net/http"
	"strings"

	"api-gateway/internal/auth"
	"api-gateway/internal/dto"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthHandler struct {
	authClient *auth.Client
	log        *zap.Logger
}

func NewAuthHandler(authClient *auth.Client, log *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authClient: authClient,
		log:        log,
	}
}

// RegisterHandler godoc
// @Summary Регистрация пользователя
// @Description Создаёт нового пользователя с ролью CUSTOMER
// @Tags auth
// @Accept json
// @Produce json
// @Param register body dto.RegisterRequest true "Данные регистрации"
// @Success 200 {object} dto.RegisterResponse "Успешная регистрация"
// @Failure 400 {object} dto.ValidationErrorResponse "Неверные данные"
// @Failure 409 {object} dto.ConflictErrorResponse "Пользователь уже существует"
// @Failure 500 {object} dto.InternalErrorResponse "Внутренняя ошибка"
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid registration request", zap.Error(err))
		verr := dto.NewValidationError("invalid request body", []dto.FieldError{})
		c.JSON(http.StatusBadRequest, verr)
		return
	}

	resp, err := h.authClient.Register(c.Request.Context(), req)
	if err != nil {
		// Попробуем распарсить gRPC статус
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.InvalidArgument:
				h.log.Warn("Validation failed at auth service", zap.String("email", req.Email), zap.Error(err))
				c.JSON(http.StatusBadRequest, dto.NewValidationError("validation failed", []dto.FieldError{}))
				return
			case codes.AlreadyExists:
				h.log.Warn("User already exists", zap.String("email", req.Email))
				c.JSON(http.StatusConflict, dto.NewConflictError("user with this email already exists"))
				return
			default:
				h.log.Error("Auth service error", zap.String("code", st.Code().String()), zap.Error(err))
				c.JSON(http.StatusInternalServerError, dto.NewInternalError(trimStatusMessage(st.Message())))
				return
			}
		}
		// Не gRPC статус — внутренняя ошибка
		h.log.Error("Registration failed (non-status error)", zap.Error(err))
		c.JSON(http.StatusInternalServerError, dto.NewInternalError(""))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// LoginHandler godoc
// @Summary Авторизация пользователя
// @Description Авторизует пользователя и выдаёт JWKs токен
// @Tags auth
// @Accept json
// @Produce json
// @Param login body dto.LoginRequest true "Данные авторизации"
// @Success 200 {object} dto.LoginResponse
// @Failure 400 {object} dto.ValidationErrorResponse "Неверные данные"
// @Failure 401 {object} dto.UnauthorizedErrorResponse "Ошибка авторизации"
// @Failure 500 {object} dto.InternalErrorResponse "Внутренная ошибка"
// @Failure 404 {object} dto.NotFoundErrorResponse "Пользователь не найден"
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid registration request", zap.Error(err))
		verr := dto.NewValidationError("invalid request body", []dto.FieldError{})
		c.JSON(http.StatusBadRequest, verr)
		return
	}

	resp, err := h.authClient.Login(c.Request.Context(), req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.InvalidArgument:
				h.log.Warn("Validation failed at auth service", zap.String("email", req.Email), zap.Error(err))
				c.JSON(http.StatusBadRequest, dto.NewValidationError("validation faild", []dto.FieldError{}))
				return
			case codes.NotFound:
				h.log.Warn("User not found", zap.String("email", req.Email))
				c.JSON(http.StatusNotFound, dto.NewNotFoundError("user with this email not found"))
				return
			case codes.Unauthenticated:
				h.log.Warn("User not authenticated", zap.String("email", req.Email))
				c.JSON(http.StatusUnauthorized, dto.NewUnauthorizedError("user not authenticated"))
				return
			default:
				h.log.Error("Internal service error", zap.String("code", st.Code().String()), zap.Error(err))
				c.JSON(http.StatusInternalServerError, dto.NewInternalError(trimStatusMessage(st.Message())))
			}
		}
		h.log.Error("Login failed (non-status error)", zap.Error(err))
		c.JSON(http.StatusInternalServerError, dto.NewInternalError(""))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func trimStatusMessage(msg string) string {
	// Убираем возможные приставки вроде "validation failed:" чтобы клиенту было чище
	lower := strings.ToLower(msg)
	if strings.HasPrefix(lower, "validation failed") {
		// вернём оригинал без префикса (если двоеточие есть)
		parts := strings.SplitN(msg, ":", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}
	return msg
}
