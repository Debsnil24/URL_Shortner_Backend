package main

import (
	"log"

	"github.com/Debsnil24/URL_Shortner.git/config"
	"github.com/Debsnil24/URL_Shortner.git/routes"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-gormigrate/gormigrate/v2"
)

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
	config.AllowOrigins = []string{"http://localhost:3000", "https://url-shortner-nine-psi.vercel.app", "https://www.sniply.co.in"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"}
	config.AllowCredentials = true
	router.Use(cors.New(config))

	routes.RegisterRoutes(router)

	router.Run(":8080")
}
