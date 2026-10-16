package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"user/database"
	user "user/internal"
	"user/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Load configuration: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "user-service",
			"status":  "healthy",
		})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		if err := db.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"service": "user-service",
				"status":  "not ready",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"service":  "user-service",
			"status":   "ready",
			"database": "connected",
		})
	})

	// User API
	repo := user.NewRepository(db)
	handler := user.NewHandler(repo)
	handler.RegisterRoutes(router)

	log.Printf("User Service starting on 127.0.0.1:%s", cfg.Port)

	if err := router.Run("127.0.0.1:" + cfg.Port); err != nil {
		log.Fatalf("Start User Service: %v", err)
	}
}
