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

	// Public redirect route
	router.GET("/:code", h.RedirectURL)

	api := router.Group("/api")
	{
		api.GET("/test", h.TestHandler)
		api.POST("/shorten", middleware.AuthRequired(), h.ShortenURL)
		api.GET("/urls", middleware.AuthRequired(), h.ListURLs)
		api.GET("/urls/:code/stats", middleware.AuthRequired(), h.GetURLStats)
		api.DELETE("/delete/:code", middleware.AuthRequired(), h.DeleteURL)
	}

	auth := router.Group("/auth", middleware.RequestTimeout(1*time.Minute))
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/logout", h.Logout)
		auth.GET("/google", h.GoogleAuth)
		auth.GET("/google/callback", h.GoogleCallback)
		auth.GET("/me", middleware.AuthRequired(), h.Me)
		// auth.POST("/refresh", h.Refresh)
	}

}
