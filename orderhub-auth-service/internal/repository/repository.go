package repository

import "gorm.io/gorm"

type Repository struct {
	DB                *gorm.DB
	Users             UserRepo
	RefreshTokens     RefreshRepo
	PasswordReset     PasswordResetRepo
	EmailVerification EmailVerificationRepo
	JWK               JWKRepo
	Session           SessionRepo
}

func buildRepository(db *gorm.DB) *Repository {
	return &Repository{
		DB:                db,
		Users:             NewUserRepo(db),
		RefreshTokens:     NewRefreshRepo(db),
		PasswordReset:     NewPasswordResetRepo(db),
		EmailVerification: NewEmailVerificationRepo(db),
		JWK:               NewJWKRepo(db),
		Session:           NewSessionRepo(db),
	}
}

func New(db *gorm.DB) *Repository { return buildRepository(db) }
