package main

import (
	"log"

	"github.com/Debsnil24/URL_Shortner.git/config"
	"github.com/Debsnil24/URL_Shortner.git/routes"
	_ "github.com/Debsnil24/URL_Shortner.git/swagger" // Swagger docs
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-gormigrate/gormigrate/v2"
)

// @title           URL Shortener API
// @version         1.0
// @description     A URL shortener service API with authentication and analytics
// @termsOfService  https://sniply.co.in/terms

// @contact.name   API Support
// @contact.url    https://sniply.co.in/support
// @contact.email  support@sniply.co.in

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	DB := config.InitDB()
	mig := gormigrate.New(DB, gormigrate.DefaultOptions, config.GetMigrations())
	if err := mig.Migrate(); err != nil {
		log.Fatalf("❌ Migration Failed: %v", err)
	}

	// Initialize Google OAuth (reads env vars)
	config.InitGoogleOAuth()

	router := gin.Default()

	// Configure CORS
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{
		"http://localhost:3000",
		"https://url-shortner-nine-psi.vercel.app",
		"https://www.sniply.co.in",
		"https://sniply.co.in",     // Add without www
		"https://dev.sniply.co.in", // Add dev environment
	}
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"}
	config.AllowCredentials = true
	router.Use(cors.New(config))

	routes.RegisterRoutes(router)

	router.Run(":8080")
}
