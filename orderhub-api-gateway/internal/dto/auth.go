package dto

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type RegisterResponse struct {
	UserId    string `json:"user_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginResponse struct {
	UserId string `json:"user_id"`
	Role   string `json:"role"`
	Tokens struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		AccessExpiresIn  int64  `json:"access_expires_in"`
		RefreshExpiresIn int64  `json:"refresh_expires_in"`
	} `json:"tokens"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type RefreshResponse struct {
	Tokens struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		AccessExpiresIn  int64  `json:"access_expires_in"`
		RefreshExpiresIn int64  `json:"refresh_expires_in"`
	} `json:"tokens"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
	All          bool   `json:"all"`
}

type GetJwksResponse struct {
	Keys []Jwk `json:"keys"`
}

type Jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type IntrospectRequest struct {
	AccessToken string `json:"access_token" binding:"required"`
}

type IntrospectResponse struct {
	Active  bool     `json:"active"`
	UserId  string   `json:"user_id"`
	Role    string   `json:"role"`
	ExpUnix int64    `json:"exp_unix"`
	Scopes  []string `json:"scopes"`
}
