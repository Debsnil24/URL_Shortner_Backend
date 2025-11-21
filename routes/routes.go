package routes

import (
	"time"

	"github.com/Debsnil24/URL_Shortner.git/config"
	"github.com/Debsnil24/URL_Shortner.git/handler"
	"github.com/Debsnil24/URL_Shortner.git/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine) {
	h := handler.NewHandler(config.DB)

	// Swagger token endpoint (protected, returns token for Swagger UI)
	// Using /api/swagger-token to avoid conflict with /swagger/*any catch-all route
	// Uses SwaggerAuthRequired to check for swagger_auth_token cookie (separate from frontend)
	router.GET("/api/swagger-token", middleware.SwaggerAuthRequired(), middleware.SwaggerTokenEndpoint())

	// Swagger UI routes (protected, requires Swagger-specific authentication)
	// Must be registered BEFORE the /:code catch-all route
	router.GET("/swagger", middleware.SwaggerAuthRequired(), func(c *gin.Context) {
		c.Redirect(302, "/swagger/index.html")
	})
	router.GET("/swagger/*any", middleware.SwaggerAuthRequired(), handler.CustomSwaggerHandler())

	api := router.Group("/api")
	{
		api.GET("/test", h.TestHandler)
		api.POST("/shorten", middleware.AuthRequired(), h.ShortenURL)
		api.GET("/urls", middleware.AuthRequired(), h.ListURLs)
		api.GET("/urls/:code/stats", middleware.AuthRequired(), h.GetURLStats)
		api.PATCH("/urls/:code/status", middleware.AuthRequired(), h.UpdateURLStatus) // Status update endpoint (must be before /urls/:code)
		api.GET("/urls/:code/qr-token", middleware.AuthRequired(), h.GetQRToken)      // QR token endpoint (must be before /urls/:code/qr)
		api.GET("/urls/:code/qr", middleware.OptionalAuth(), h.GetQRCode)             // QR code retrieval (must be before /urls/:code) - supports both cookie and QR token auth
		api.POST("/urls/:code/qr", middleware.AuthRequired(), h.GenerateQRCode)       // QR code generation (must be before /urls/:code)
		api.PATCH("/urls/:code", middleware.AuthRequired(), h.UpdateURL)
		api.DELETE("/delete/:code", middleware.AuthRequired(), h.DeleteURL)
		// Support endpoint with rate limiting and timeout
		api.POST("/support", middleware.RateLimit(), middleware.RequestTimeout(30*time.Second), h.SubmitSupport)
	}

	auth := router.Group("/auth", middleware.RequestTimeout(1*time.Minute))
	{
		auth.GET("/login-page", handler.LoginPage()) // Backend login page for Swagger
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/logout", h.Logout)                // Handles both frontend and Swagger logout
		auth.POST("/swagger-logout", h.SwaggerLogout) // Swagger-specific logout endpoint (no auth required)
		auth.GET("/google", h.GoogleAuth)
		auth.GET("/google/callback", h.GoogleCallback)
		auth.GET("/me", middleware.AuthRequired(), h.Me)
		// auth.POST("/refresh", h.Refresh)
	}

	// Public redirect route (MUST be last to avoid catching other routes like /swagger, /api, /auth)
	// Support both GET and HEAD methods
	router.GET("/:code", h.RedirectURL)
	router.HEAD("/:code", h.RedirectURL)

}
