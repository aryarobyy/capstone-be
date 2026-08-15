package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"capstone-be/config"
	"capstone-be/internal/database"
	"capstone-be/internal/middleware"
	"capstone-be/internal/modules/health"
	"capstone-be/internal/modules/user"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Load Configurations
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configurations: %v", err)
	}

	// 2. Initialize Database Connection
	db, err := database.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	if db != nil {
		defer func() {
			log.Println("Closing database connection...")
			if err := db.Close(); err != nil {
				log.Printf("Error closing database: %v", err)
			}
		}()
	}

	// 3. Set Gin mode
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 4. Setup Router and Middlewares
	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS())

	// 5. Setup Route Groups
	apiGroup := r.Group("/api")
	v1Group := apiGroup.Group("/v1")

	// 6. Register Module Routes
	health.RegisterRoutes(v1Group, db)
	user.RegisterRoutes(v1Group, db)

	// 7. Setup Server & Graceful Shutdown
	serverAddr := ":" + cfg.Port
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server running on port %s in %s mode\n", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Listen and serve error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Allow server 5 seconds to finish active connections
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting successfully")
}
