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
	"capstone-be/internal/modules/auth"
	"capstone-be/internal/modules/health"
	"capstone-be/internal/modules/history"
	"capstone-be/internal/modules/sensor"
	"capstone-be/internal/modules/user"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configurations: %v", err)
	}

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

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS())

	apiGroup := r.Group("/api")

	health.RegisterRoutes(apiGroup, db)
	auth.RegisterRoutes(apiGroup, db)
	user.RegisterRoutes(apiGroup, db)
	history.RegisterRoutes(apiGroup, db)
	sensor.RegisterRoutes(apiGroup, db)

	serverAddr := ":" + cfg.Port
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: r,
	}

	go func() {
		log.Printf("Server running on port %s in %s mode\n", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Listen and serve error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting successfully")
}
