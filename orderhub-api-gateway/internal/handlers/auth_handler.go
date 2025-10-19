package handlers

import (
	"net/http"
	"strings"

	"api-gateway/internal/auth"
	"api-gateway/internal/dto"
	"api-gateway/internal/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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
// @Description Авторизует пользователя и выдаёт пару токенов (access/refresh)
// @Tags auth
// @Accept json
// @Produce json
// @Param login body dto.LoginRequest true "Данные авторизации"
// @Success 200 {object} dto.LoginResponse
// @Failure 400 {object} dto.ValidationErrorResponse "Неверные данные"
// @Failure 401 {object} dto.UnauthorizedErrorResponse "Ошибка авторизации"
// @Failure 500 {object} dto.InternalErrorResponse "Внутренняя ошибка"
// @Failure 404 {object} dto.NotFoundErrorResponse "Пользователь не найден"
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid login request", zap.Error(err))
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
				c.JSON(http.StatusBadRequest, dto.NewValidationError("validation failed", []dto.FieldError{}))
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
				return
			}
		}
		h.log.Error("Login failed (non-status error)", zap.Error(err))
		c.JSON(http.StatusInternalServerError, dto.NewInternalError(""))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// RefreshHandler godoc
// @Summary Обновление токена
// @Description Обновляет пару токенов по refresh токену
// @Tags auth
// @Accept json
// @Produce json
// @Param refresh body dto.RefreshRequest true "Данные для обновления токена"
// @Success 200 {object} dto.RefreshResponse
// @Failure 400 {object} dto.ValidationErrorResponse "Неверные данные"
// @Failure 401 {object} dto.UnauthorizedErrorResponse "Ошибка авторизации"
// @Failure 500 {object} dto.InternalErrorResponse "Внутренняя ошибка"
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid refresh request", zap.Error(err))
		verr := dto.NewValidationError("invalid request body", []dto.FieldError{})
		c.JSON(http.StatusBadRequest, verr)
		return
	}

	resp, err := h.authClient.Refresh(c.Request.Context(), req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.InvalidArgument:
				h.log.Warn("Validation failed at auth service", zap.String("refresh_token", req.RefreshToken), zap.Error(err))
				c.JSON(http.StatusBadRequest, dto.NewValidationError("validation failed", []dto.FieldError{}))
				return
			case codes.NotFound:
				h.log.Warn("Refresh token not found", zap.String("refresh_token", req.RefreshToken))
				c.JSON(http.StatusNotFound, dto.NewNotFoundError("user with this refresh token not found"))
				return
			case codes.Unauthenticated:
				h.log.Warn("User not authenticated", zap.String("refresh_token", req.RefreshToken))
				c.JSON(http.StatusUnauthorized, dto.NewUnauthorizedError("user not authenticated"))
				return
			default:
				h.log.Error("Internal service error", zap.String("code", st.Code().String()), zap.Error(err))
				c.JSON(http.StatusInternalServerError, dto.NewInternalError(trimStatusMessage(st.Message())))
			}
		}
		h.log.Error("Refresh failed (non-status error)", zap.Error(err))
		c.JSON(http.StatusInternalServerError, dto.NewInternalError(""))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// RequestPasswordResetHandler godoc
// @Summary Запрос на сброс пароля
// @Description Запрашивает сброс пароля для пользователя по почте
// @Tags auth
// @Accept json
// @Produce json
// @Param email body dto.RequestPasswordResetRequest true "Email пользователя"
// @Success 200 {object} dto.SuccessResponse "Успешный запрос на сброс пароля"
// @Failure 400 {object} dto.ValidationErrorResponse "Неверные данные"
// @Failure 404 {object} dto.NotFoundErrorResponse "Пользователь не найден"
// @Failure 500 {object} dto.InternalErrorResponse "Внутренняя ошибка"
// @Router /api/v1/auth/request-password-reset [post]
func (h *AuthHandler) RequestPasswordReset(c *gin.Context) {
	var req dto.RequestPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid request password request", zap.Error(err))
		verr := dto.NewValidationError("invalid request body", []dto.FieldError{})
		c.JSON(http.StatusBadRequest, verr)
		return
	}

	err := h.authClient.RequestPasswordReset(c.Request.Context(), req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				h.log.Warn("Password reset request failed (user not found)", zap.String("email", req.Email))
				c.JSON(http.StatusNotFound, dto.NewNotFoundError("password reset request failed (user not found)"))
				return
			case codes.ResourceExhausted:
				h.log.Warn("Too many requests", zap.String("email", req.Email))
				c.JSON(http.StatusTooManyRequests, dto.NewTooManyRequestsError("too many requests"))
				return
			default:
				h.log.Error("Internal service error", zap.String("code", st.Code().String()), zap.Error(err))
				c.JSON(http.StatusInternalServerError, dto.NewInternalError(trimStatusMessage(st.Message())))
				return
			}
		}
		// Не gRPC статус — внутренняя ошибка
		h.log.Error("Password reset request failed (non-status error)", zap.Error(err))
		c.JSON(http.StatusInternalServerError, dto.NewInternalError(""))
		return
	}

	c.JSON(http.StatusOK, dto.NewSuccessResponse("password reset requested"))
}

// ConfirmPasswordResetHandler godoc
// @Summary Подтверждение сброса пароля
// @Description Подтверждает сброс пароля для пользователя
// @Tags auth
// @Accept json
// @Produce json
// @Param code body dto.ConfirmPasswordResetRequest true "Код подтверждения и новый пароль"
// @Success 200 {object} dto.SuccessResponse "Успешное подтверждение сброса пароля"
// @Failure 400 {object} dto.ValidationErrorResponse "Неверные данные"
// @Failure 404 {object} dto.NotFoundErrorResponse "Пользователь не найден"
// @Failure 500 {object} dto.InternalErrorResponse "Внутренняя ошибка"
// @Router /api/v1/auth/confirm-password-reset [post]
func (h *AuthHandler) ConfirmPasswordReset(c *gin.Context) {
	var req dto.ConfirmPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid confirm password reset request", zap.Error(err))
		verr := dto.NewValidationError("invalid request body", []dto.FieldError{})
		c.JSON(http.StatusBadRequest, verr)
		return
	}

	err := h.authClient.ConfirmPasswordReset(c.Request.Context(), req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				h.log.Warn("Password reset confirm failed (code not found)", zap.String("code", req.Code))
				c.JSON(http.StatusNotFound, dto.NewNotFoundError("password reset confirm failed (code not found)"))
				return
			case codes.InvalidArgument:
				h.log.Warn("Password reset confirm failed (invalid argument)", zap.String("code", req.Code))
				c.JSON(http.StatusBadRequest, dto.NewValidationError("password reset confirm failed (invalid argument)", []dto.FieldError{}))
				return
			default:
				h.log.Error("Internal service error", zap.String("code", st.Code().String()), zap.Error(err))
				c.JSON(http.StatusInternalServerError, dto.NewInternalError(trimStatusMessage(st.Message())))
				return
			}
		}
		h.log.Error("Confirm password reset failed (non-status error)", zap.Error(err))
		c.JSON(http.StatusInternalServerError, dto.NewInternalError(""))
		return
	}

	c.JSON(http.StatusOK, dto.NewSuccessResponse("password reset confirmed"))
}

// GetJwksHandler godoc
// @Summary Получение JWKS
// @Description Получает JSON Web Key Set (JWKS) для проверки подписи JWT
// @Tags auth
// @Router /api/v1/auth/jwks [get]
func (h *AuthHandler) GetJwks(c *gin.Context) {
	resp, err := h.authClient.GetJwks(c.Request.Context())
	if err != nil {
		h.log.Error("Failed to get JWKS", zap.Error(err))
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

// LogoutHandler godoc
// @Summary Выход из системы
// @Description Логаут по refresh_token (single) или массовый логаут по all=true
// @Security BearerAuth
// @Tags auth
// @Accept json
// @Produce json
// @Param logout body dto.LogoutRequest true "Refresh token или all=true"
// @Success 200 {object} dto.SuccessResponse "Успешный логаут"
// @Failure 400 {object} dto.ValidationErrorResponse "Неверные данные"
// @Failure 404 {object} dto.NotFoundErrorResponse "Токен не найден"
// @Failure 500 {object} dto.InternalErrorResponse "Внутренняя ошибка"
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req dto.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid logout request", zap.Error(err))
		c.JSON(http.StatusBadRequest, dto.NewValidationError("invalid request body", []dto.FieldError{}))
		return
	}

	if strings.TrimSpace(req.RefreshToken) == "" && !req.All {
		c.JSON(http.StatusBadRequest, dto.NewValidationError("specify refresh_token or all=true", []dto.FieldError{}))
		return
	}

	ctx := c.Request.Context()
	if authz := c.GetHeader("Authorization"); strings.TrimSpace(authz) != "" {
		if token, ok := middleware.ExtractBearerToken(authz); ok && token != "" {
			// Собираем корректный заголовок без мусора
			md := metadata.Pairs("authorization", "Bearer "+token)
			ctx = metadata.NewOutgoingContext(ctx, md)
		}
	}

	err := h.authClient.Logout(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.InvalidArgument:
				c.JSON(http.StatusBadRequest, dto.NewValidationError(trimStatusMessage(st.Message()), []dto.FieldError{}))
				return
			case codes.NotFound:
				c.JSON(http.StatusNotFound, dto.NewNotFoundError("refresh token not found or revoked"))
				return
			default:
				h.log.Error("Internal service error", zap.String("code", st.Code().String()), zap.Error(err))
				c.JSON(http.StatusInternalServerError, dto.NewInternalError(trimStatusMessage(st.Message())))
				return
			}
		}
		h.log.Error("Logout failed (non-status error)", zap.Error(err))
		c.JSON(http.StatusInternalServerError, dto.NewInternalError(""))
		return
	}

	c.JSON(http.StatusOK, dto.NewSuccessResponse("logged out"))
}

// RequestEmailVerificationHandler godoc
// @Summary Повторная отправка письма подтверждения
// @Description Отправляет письмо подтверждения для текущего авторизованного пользователя
// @Security BearerAuth
// @Tags auth
// @Produce json
// @Success 200 {object} dto.SuccessResponse "Письмо отправлено"
// @Failure 401 {object} dto.UnauthorizedErrorResponse "Неавторизован"
// @Failure 404 {object} dto.NotFoundErrorResponse "Пользователь не найден"
// @Failure 409 {object} dto.ConflictErrorResponse "Email уже подтверждён"
// @Failure 429 {object} dto.TooManyRequestsErrorResponse "Слишком много запросов"
// @Failure 500 {object} dto.InternalErrorResponse "Внутренняя ошибка"
// @Router /api/v1/auth/email/verification/request [post]
func (h *AuthHandler) RequestEmailVerification(c *gin.Context) {
	// Требует авторизации: берём токен из заголовка и пробрасываем в gRPC
	ctx := c.Request.Context()
	if authz := c.GetHeader("Authorization"); strings.TrimSpace(authz) != "" {
		if token, ok := middleware.ExtractBearerToken(authz); ok && token != "" {
			md := metadata.Pairs("authorization", "Bearer "+token)
			ctx = metadata.NewOutgoingContext(ctx, md)
		} else {
			c.JSON(http.StatusUnauthorized, dto.NewUnauthorizedError("invalid Authorization header"))
			return
		}
	} else {
		c.JSON(http.StatusUnauthorized, dto.NewUnauthorizedError("missing Authorization header"))
		return
	}

	if err := h.authClient.RequestEmailVerification(ctx); err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.NotFound:
				h.log.Warn("User not found for email verification request")
				c.JSON(http.StatusNotFound, dto.NewNotFoundError("user not found"))
				return
			case codes.FailedPrecondition:
				h.log.Warn("Email already verified")
				c.JSON(http.StatusConflict, dto.NewConflictError("email already verified"))
				return
			case codes.ResourceExhausted:
				// Может означать лимит частоты или уже идёт процесс верификации
				h.log.Warn("Email verification request rate limited or in progress")
				c.JSON(http.StatusTooManyRequests, dto.NewTooManyRequestsError("too many requests"))
				return
			default:
				h.log.Error("Internal service error", zap.String("code", st.Code().String()), zap.Error(err))
				c.JSON(http.StatusInternalServerError, dto.NewInternalError(trimStatusMessage(st.Message())))
				return
			}
		}
		h.log.Error("Request email verification failed (non-status error)", zap.Error(err))
		c.JSON(http.StatusInternalServerError, dto.NewInternalError(""))
		return
	}

	c.JSON(http.StatusOK, dto.NewSuccessResponse("verification email requested"))
}

// ConfirmEmailVerificationHandler godoc
// @Summary Подтверждение email по коду
// @Description Подтверждает почту по одноразовому коду из письма
// @Tags auth
// @Accept json
// @Produce json
// @Param confirm body dto.ConfirmEmailVerificationRequest true "Код подтверждения"
// @Success 200 {object} dto.SuccessResponse "Email подтверждён"
// @Failure 400 {object} dto.ValidationErrorResponse "Неверные данные или истёкший код"
// @Failure 404 {object} dto.NotFoundErrorResponse "Пользователь не найден"
// @Failure 500 {object} dto.InternalErrorResponse "Внутренняя ошибка"
// @Router /api/v1/auth/email/verification/confirm [post]
func (h *AuthHandler) ConfirmEmailVerification(c *gin.Context) {
	var req dto.ConfirmEmailVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid confirm email verification request", zap.Error(err))
		c.JSON(http.StatusBadRequest, dto.NewValidationError("invalid request body", []dto.FieldError{}))
		return
	}

	if err := h.authClient.ConfirmEmailVerification(c.Request.Context(), req); err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.InvalidArgument:
				h.log.Warn("Invalid or expired verification code")
				c.JSON(http.StatusBadRequest, dto.NewValidationError("invalid or expired code", []dto.FieldError{}))
				return
			case codes.NotFound:
				h.log.Warn("User not found for email confirmation")
				c.JSON(http.StatusNotFound, dto.NewNotFoundError("user not found"))
				return
			default:
				h.log.Error("Internal service error", zap.String("code", st.Code().String()), zap.Error(err))
				c.JSON(http.StatusInternalServerError, dto.NewInternalError(trimStatusMessage(st.Message())))
				return
			}
		}
		h.log.Error("Confirm email verification failed (non-status error)", zap.Error(err))
		c.JSON(http.StatusInternalServerError, dto.NewInternalError(""))
		return
	}

	c.JSON(http.StatusOK, dto.NewSuccessResponse("email verified"))
}
