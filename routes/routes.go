package routes

import (
	"github.com/Debsnil24/URL_Shortner.git/config"
	"github.com/Debsnil24/URL_Shortner.git/handler"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine) {
	h := handler.NewHandler(config.DB)

	api := router.Group("/api")
	{
		api.GET("/test", h.TestHandler)
		api.POST("/shorten", h.ShortenURL)
		// api.GET("/:code", h.RedirectURL)
		// api.GET("/stats/:code", h.GetStats)
		api.DELETE("/delete/:code", h.DeleteURL)
	}

	// auth := router.Group("/auth")
	// {
	// 	auth.POST("/register", h.Register)
	// 	auth.POST("/login", h.Login)
	// 	auth.GET("/me", h.AuthCheck)
	// }

}
