package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"

	"github.com/gin-gonic/gin"

	"user/database"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env not found; using default values")
	}

	db, err := database.Connect()
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	defer db.Close()

	log.Println("Connected to Postgresql correctly")

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "user is healthy",
		})
	})

	router.GET("/api/v1/db-check", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(
			c.Request.Context(),
			3*time.Second,
		)
		defer cancel()

		var databaseName string

		err := db.QueryRowContext(
			ctx,
			"SELECT current_database()",
		).Scan(&databaseName)

		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Database query failed",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"database": databaseName,
			"status":   "Database connection is healthy",
		})
	})

	router.GET("/api/v1/users", func(c *gin.Context) {
		rows, err := db.QueryContext(
			c.Request.Context(),
			`SELECT id, full_name, created_at
			FROM users
			ORDER BY id`,
		)

		if err != nil {
			log.Printf("failed to query users: %v", err)

			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve users",
			})
			return
		}
		defer rows.Close()

		type User struct {
			ID        int64     `json:"id"`
			FullName  string    `json:"full_name"`
			CreatedAt time.Time `json:"created_at"`
		}

		users := make([]User, 0)

		for rows.Next() {
			var user User

			if err := rows.Scan(
				&user.ID,
				&user.FullName,
				&user.CreatedAt,
			); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to read user data",
				})
				return
			}

			users = append(users, user)
		}

		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to iterate over users",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"count": len(users),
			"users": users,
		})
	})

	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
