package router

import (
	"github.com/uuuuupdate/gh-proxy-auth/internal/handlers"
	"github.com/uuuuupdate/gh-proxy-auth/internal/middleware"
	"github.com/gin-gonic/gin"
)

func Setup(engine *gin.Engine) {
	engine.Use(middleware.CORS())

	authHandler := handlers.NewAuthHandler()
	userHandler := handlers.NewUserHandler()
	tokenHandler := handlers.NewTokenHandler()
	adminHandler := handlers.NewAdminHandler()
	systemHandler := handlers.NewSystemHandler()
	updateHandler := handlers.NewUpdateHandler()
	proxyHandler := handlers.NewProxyHandler()

	// System APIs (no auth required)
	api := engine.Group("/api")
	{
		api.GET("/system/init-status", systemHandler.GetInitStatus)

		// Auth APIs
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/totp/verify", authHandler.VerifyTOTP)
			auth.POST("/passkey/begin-login", authHandler.BeginPasskeyLogin)
			auth.POST("/passkey/finish-login", authHandler.FinishPasskeyLogin)
		}

		// User APIs (require JWT auth)
		user := api.Group("/user")
		user.Use(middleware.JWTAuth())
		{
			user.GET("/profile", userHandler.GetProfile)
			user.PUT("/password", userHandler.ChangePassword)
			user.POST("/totp/setup", userHandler.SetupTOTP)
			user.POST("/totp/enable", userHandler.EnableTOTP)
			user.DELETE("/totp", userHandler.DisableTOTP)
			user.PUT("/mfa-priority", userHandler.SetMFAPriority)
			user.GET("/passkeys", userHandler.ListPasskeys)
			user.POST("/passkey/begin-register", userHandler.BeginRegisterPasskey)
			user.POST("/passkey/finish-register", userHandler.FinishRegisterPasskey)
			user.DELETE("/passkey/:id", userHandler.DeletePasskey)
		}

		// Token APIs (require JWT auth)
		tokens := api.Group("/tokens")
		tokens.Use(middleware.JWTAuth())
		{
			tokens.GET("", tokenHandler.List)
			tokens.POST("", tokenHandler.Create)
			tokens.PUT("/:id", tokenHandler.Update)
			tokens.DELETE("/:id", tokenHandler.Delete)
			tokens.GET("/:id/logs", tokenHandler.GetLogs)
		}

		// Admin APIs (require JWT auth + admin)
		admin := api.Group("/admin")
		admin.Use(middleware.JWTAuth(), middleware.AdminOnly())
		{
			admin.GET("/settings", adminHandler.GetSettings)
			admin.PUT("/settings", adminHandler.UpdateSettings)
			admin.GET("/users", adminHandler.ListUsers)
			admin.PUT("/users/:id/speed-limit", adminHandler.UpdateUserSpeedLimit)
			admin.GET("/logs", adminHandler.GetDownloadLogs)
			admin.GET("/update/check", updateHandler.CheckUpdate)
			admin.POST("/update/apply", updateHandler.ApplyUpdate)
		}
	}

	// Proxy handler - catch all non-API, non-frontend routes
	engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Check if this is a proxy request (GitHub URL)
		trimmedPath := path[1:] // remove leading /
		if handlers.IsProxyPath(trimmedPath) {
			proxyHandler.Handle(c)
			return
		}

		// Serve frontend
		handlers.ServeFrontend(c)
	})
}
