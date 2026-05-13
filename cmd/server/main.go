// backend/cmd/server/main.go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/freshtea599/PhotoHubServer.git/internal/auth"
	"github.com/freshtea599/PhotoHubServer.git/internal/config"
	"github.com/freshtea599/PhotoHubServer.git/internal/repository"
	customhttp "github.com/freshtea599/PhotoHubServer.git/internal/transport/http"
	"github.com/freshtea599/PhotoHubServer.git/internal/usecase"
	vipsproc "github.com/freshtea599/PhotoHubServer.git/pkg/vips"
)

func init() {
	_ = godotenv.Load()
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// PostgreSQL
	dbURL := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)
	database, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()
	if err := database.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("✅ Connected to PostgreSQL")

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to Redis: %v", err)
	}
	log.Println("✅ Connected to Redis")

	// MinIO
	minioClient, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseSSL,
	})
	if err != nil {
		log.Fatalf("failed to create MinIO client: %v", err)
	}
	buckets := []string{cfg.MinIOBucketOriginals, cfg.MinIOBucketVariants}
	for _, bucket := range buckets {
		exists, errBucket := minioClient.BucketExists(context.Background(), bucket)
		if errBucket != nil {
			log.Fatalf("failed to check MinIO bucket %s: %v", bucket, errBucket)
		}
		if !exists {
			err = minioClient.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{})
			if err != nil {
				log.Fatalf("failed to create MinIO bucket %s: %v", bucket, err)
			}
			log.Printf("✅ Created MinIO bucket: %s", bucket)
		}
	}
	log.Println("✅ Connected to MinIO")

	// Репозитории
	photoRepo := repository.NewPostgresPhotoRepo(database)
	userRepo := repository.NewPostgresUserRepo(database)
	minioRepo := repository.NewMinioRepo(minioClient, cfg.MinIOBucketOriginals, cfg.MinIOBucketVariants)
	redisRepo := repository.NewRedisRepo(rdb)

	// VIPS
	vipsProcessor, err := vipsproc.NewProcessor()
	if err != nil {
		log.Fatalf("failed to init vips processor: %v", err)
	}

	// ImageProcessor (WorkerPool внутри)
	imageProcessor, err := usecase.NewImageProcessor(
		cfg.WorkerCount,
		vipsProcessor,
		minioRepo,
		redisRepo,
		photoRepo,
	)
	if err != nil {
		log.Fatalf("failed to create image processor: %v", err)
	}
	defer imageProcessor.Shutdown()

	// JWT
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)

	// Папка uploads (если нужна)
	if err := os.MkdirAll("uploads", 0755); err != nil {
		log.Fatalf("failed to create uploads directory: %v", err)
	}

	// HTTP сервер
	server := customhttp.NewServer(
		cfg,
		jwtManager,
		userRepo,
		photoRepo,
		minioRepo,
		rdb,
		minioClient,
		imageProcessor,
	)

	// Prometheus
	go func() {
		metricsAddr := fmt.Sprintf(":%d", cfg.PrometheusPort)
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("📊 Prometheus metrics available at %s/metrics", metricsAddr)
		if err := http.ListenAndServe(metricsAddr, nil); err != nil {
			log.Printf("prometheus server error: %v", err)
		}
	}()

	// Старт сервера
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("🚀 Starting server on %s in %s mode", addr, cfg.Env)
	go func() {
		if err := server.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}
	log.Println("Server stopped gracefully")
}
