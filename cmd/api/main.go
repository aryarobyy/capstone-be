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
	"capstone-be/internal/alert"
	"capstone-be/internal/database"
	"capstone-be/internal/middleware"
	"capstone-be/internal/modules/auth"
	"capstone-be/internal/modules/health"
	"capstone-be/internal/modules/history"
	"capstone-be/internal/modules/sensor"
	sensorreading "capstone-be/internal/modules/sensor_reading"
	"capstone-be/internal/modules/user"
	"capstone-be/internal/notification"
	"capstone-be/internal/session"
	"capstone-be/internal/token"

	"github.com/gin-gonic/gin"
)

func main() {
	closeLogger, err := middleware.InitLogger()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer closeLogger()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configurations: %v", err)
	}

	tokens, err := token.NewManager(cfg.JWTSecret, cfg.JWTExpirationHours)
	if err != nil {
		log.Fatalf("Invalid JWT configuration: %v", err)
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

	sessions := session.NewRepository(db)
	notificationRepo := notification.NewRepository(db)
	notificationService := notification.NewService(notificationRepo)
	_ = notificationService

	var worker *notification.Worker
	if os.Getenv("FCM_ENABLED") == "true" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		client, err := notification.NewMessaging(ctx, os.Getenv("FIREBASE_PROJECT_ID"))
		cancel()
		if err != nil {
			log.Fatalf("Failed to initialize FCM: %v", err)
		}
		worker = notification.NewWorker(db, client, sessions)
	} else {
		log.Println("FCM disabled; set FCM_ENABLED=true to enable push notifications")
		worker = notification.NewWorker(db, nil, sessions)
	}

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Fatal(err)
	}
	r.Use(middleware.BodyLimit())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS())

	apiGroup := r.Group("/api")

	health.RegisterRoutes(apiGroup, db)
	auth.RegisterRoutes(apiGroup, db, tokens, sessions)
	protected := apiGroup.Group("")
	protected.Use(middleware.JWTAuth(tokens, sessions), middleware.ResourceAccess(db))
	user.RegisterRoutes(protected, db)
	sensor.RegisterRoutes(protected, db)
	sensorreading.RegisterRoutes(protected, db)
	history.RegisterRoutes(protected, db)
	configured := apiGroup.Group("")
	configured.Use(middleware.JWTAuth(tokens, sessions))
	alert.RegisterRoutes(configured, db)
	ingest := sensorreading.NewSensorReadingHandler(sensorreading.NewSensorReadingService(sensorreading.NewSensorReadingRepository(db)))
	apiGroup.POST("/ingest/sensor-reading", middleware.RateLimit(600, time.Minute), middleware.SensorKey(db), ingest.Create)
	workerCtx, stopWorker := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); worker.Run(workerCtx) }()

	calcInterval := 1 * time.Minute
	if intervalStr := os.Getenv("SENSOR_CALC_INTERVAL"); intervalStr != "" {
		if d, err := time.ParseDuration(intervalStr); err == nil && d > 0 {
			calcInterval = d
		}
	}
	sensorReadingRepo := sensorreading.NewSensorReadingRepository(db)
	sensorCalcWorker := sensorreading.NewPeriodicCalcWorker(sensorReadingRepo, calcInterval, 10)
	go sensorCalcWorker.Run(workerCtx)

	defer stopWorker()

	serverAddr := ":" + cfg.Port
	srv := &http.Server{
		Addr:              serverAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       60 * time.Second,
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

	stopWorker()
	select {
	case <-workerDone:
	case <-time.After(30 * time.Second):
		log.Println("worker shutdown timed out; lease will be recovered")
	}
	log.Println("Server exiting successfully")
}
