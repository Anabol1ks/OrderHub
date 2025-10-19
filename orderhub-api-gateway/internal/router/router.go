package router

import (
	"api-gateway/internal/auth"
	"api-gateway/internal/handlers"
	"api-gateway/internal/middleware"

	"github.com/gin-contrib/cors"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
)

func Router(authClient *auth.Client, log *zap.Logger) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	authHandler := handlers.NewAuthHandler(authClient, log)
	auth := r.Group("/api/v1/auth")

	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/refresh", authHandler.Refresh)
	auth.POST("/request-password-reset", authHandler.RequestPasswordReset)
	auth.POST("/confirm-password-reset", authHandler.ConfirmPasswordReset)
	auth.GET("/jwks", authHandler.GetJwks)
	// защищаем logout валидным access-токеном
	auth.POST("/logout", middleware.AuthRequired(authClient, log), authHandler.Logout)

	// email verification
	r.POST("/api/v1/auth/email/verification/confirm", authHandler.ConfirmEmailVerification)
	auth.POST("/email/verification/request", middleware.AuthRequired(authClient, log), authHandler.RequestEmailVerification)

	return r
}
