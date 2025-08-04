package main

import (
	"log"

	"github.com/Debsnil24/URL_Shortner.git/config"
	"github.com/Debsnil24/URL_Shortner.git/routes"
	"github.com/gin-gonic/gin"
	"github.com/go-gormigrate/gormigrate/v2"
)

func main() {
	DB := config.InitDB()
	mig := gormigrate.New(DB, gormigrate.DefaultOptions, config.GetMigrations())
	if err := mig.Migrate(); err != nil {
		log.Fatalf("❌ Migration Failed: %v", err)
	}

	router := gin.Default() 

	routes.RegisterRoutes(router)

	router.Run(":8080")
}