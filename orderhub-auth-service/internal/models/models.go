package models

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleCustomer Role = "ROLE_CUSTOMER"
	RoleVendor   Role = "ROLE_VENDOR"
	RoleAdmin    Role = "ROLE_ADMIN"
)

type User struct {
	ID              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email           string    `gorm:"not null"` // уникальность обеспечим функциональным индексом lower(email)
	Password        string    `gorm:"not null"` // hash (argon2id/bcrypt)
	Role            Role      `gorm:"type:text;not null;default:'ROLE_CUSTOMER';index"`
	IsEmailVerified bool      `gorm:"not null;default:false;index"`
	CreatedAt       time.Time `gorm:"not null;default:now()"`
	UpdatedAt       time.Time `gorm:"not null;default:now()"`
}

func (User) TableName() string { return "users" }

type RefreshToken struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index"`
	TokenHash  string    `gorm:"not null;index"` // хранить ХЭШ refresh (opaque)
	ClientID   *string   `gorm:"type:text"`
	IP         *string   `gorm:"type:inet"`
	UserAgent  *string   `gorm:"type:text"`
	ExpiresAt  time.Time `gorm:"not null;index"`
	Revoked    bool      `gorm:"not null;default:false;index"`
	CreatedAt  time.Time `gorm:"not null;default:now()"`
	LastUsedAt *time.Time
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

// На вырост: хранение и ротация RSA-ключей для JWKS.
// Если приватный ключ держишь вне БД — убери поле PrivPEM.
type JwkKey struct {
	KID       string     `gorm:"primaryKey;size:128"`
	Alg       string     `gorm:"type:text;not null;default:'RS256'"`
	Kty       string     `gorm:"type:text;not null;default:'RSA'"`
	Use       string     `gorm:"type:text;not null;default:'sig'"`
	N         string     `gorm:"type:text;not null"`
	E         string     `gorm:"type:text;not null"`
	PrivPEM   []byte     `gorm:"type:bytea;not null"`
	Active    bool       `gorm:"not null;default:false;index"`
	CreatedAt time.Time  `gorm:"not null;default:now()"`
	RotatesAt *time.Time `gorm:"index"`
}

func (JwkKey) TableName() string { return "jwk_keys" }

type EmailVerification struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Email     string    `gorm:"not null"`
	CodeHash  string    `gorm:"not null;index"`
	ExpiresAt time.Time `gorm:"not null;index"`
	Consumed  bool      `gorm:"not null;default:false;index"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
}

func (EmailVerification) TableName() string { return "email_verifications" }

type PasswordResetToken struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Email     string    `gorm:"not null"`
	CodeHash  string    `gorm:"not null;index"`
	ExpiresAt time.Time `gorm:"not null;index"`
	Consumed  bool      `gorm:"not null;default:false;index"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
}

func (PasswordResetToken) TableName() string { return "password_reset_tokens" }

type UserSession struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index"`
	ClientID   string    `gorm:"type:text;index"` // device/browser fingerprint
	IP         *string   `gorm:"type:inet"`
	UserAgent  *string   `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"not null;default:now()"`
	LastSeenAt time.Time `gorm:"not null;default:now();index"`
	Revoked    bool      `gorm:"not null;default:false;index"`
}

func (UserSession) TableName() string { return "user_sessions" }
