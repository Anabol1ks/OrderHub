package dto

// BaseError универсальный корневой формат ошибки
// Code — машинно-ориентированный код (snake_case)
// Message — краткое человеко-читаемое описание (может локализоваться на клиенте)
// Details — дополнительная строка (stack / пояснение / fragment)
// Fields — для валидационных ошибок (имя поля + текст)
type BaseError struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details string       `json:"details,omitempty"`
	Fields  []FieldError `json:"fields,omitempty"`
}

// FieldError отдельная ошибка по конкретному полю
// Field: путь к полю (например: "email" или "address.city")
// Message: описание нарушения
// Tag: (опционально) исходный тег валидатора (min/email/required)
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Tag     string `json:"tag,omitempty"`
}

// Предопределённые обёртки (семантические типы) — можно использовать в swagger для разных @Failure
// Все они совместимы по JSON (поля те же), просто повышают читаемость документации.

// ValidationErrorResponse 400
// Пример: неверный email или короткий пароль
// Code: "validation_error"
type ValidationErrorResponse BaseError

// ConflictErrorResponse 409
// Пример: пользователь уже существует
// Code: "conflict"
type ConflictErrorResponse BaseError

// UnauthorizedErrorResponse 401
// Пример: отсутствует/неверный токен
// Code: "unauthorized"
type UnauthorizedErrorResponse BaseError

// ForbiddenErrorResponse 403
// Пример: роль не допускает доступ
// Code: "forbidden"
type ForbiddenErrorResponse BaseError

// NotFoundErrorResponse 404
// Пример: ресурс не найден
// Code: "not_found"
type NotFoundErrorResponse BaseError

// RateLimitedErrorResponse 429
// Пример: превышен лимит запросов
// Code: "rate_limited"
type RateLimitedErrorResponse BaseError

// InternalErrorResponse 500
// Пример: внутренняя ошибка сервера
// Code: "internal_error"
type InternalErrorResponse BaseError

// Helper-функции для быстрого создания
func NewValidationError(msg string, fields []FieldError) ValidationErrorResponse {
	return ValidationErrorResponse(BaseError{Code: "validation_error", Message: msg, Fields: fields})
}
func NewConflictError(msg string) ConflictErrorResponse {
	return ConflictErrorResponse(BaseError{Code: "conflict", Message: msg})
}
func NewUnauthorizedError(msg string) UnauthorizedErrorResponse {
	return UnauthorizedErrorResponse(BaseError{Code: "unauthorized", Message: msg})
}
func NewForbiddenError(msg string) ForbiddenErrorResponse {
	return ForbiddenErrorResponse(BaseError{Code: "forbidden", Message: msg})
}
func NewNotFoundError(msg string) NotFoundErrorResponse {
	return NotFoundErrorResponse(BaseError{Code: "not_found", Message: msg})
}
func NewRateLimitedError(msg string) RateLimitedErrorResponse {
	return RateLimitedErrorResponse(BaseError{Code: "rate_limited", Message: msg})
}
func NewInternalError(details string) InternalErrorResponse {
	return InternalErrorResponse(BaseError{Code: "internal_error", Message: "internal server error", Details: details})
}
