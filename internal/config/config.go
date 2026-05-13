// internal/config/config.go
package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port int    // SERVER_PORT
	Env  string // SERVER_ENV

	DBHost     string // DB_HOST
	DBPort     int    // DB_PORT
	DBUser     string // DB_USER
	DBPassword string // DB_PASSWORD
	DBName     string // DB_NAME

	JWTSecret string // JWT_SECRET

	// Redis
	RedisAddr     string // REDIS_ADDR (host:port)
	RedisPassword string // REDIS_PASSWORD (можно пустой)
	RedisDB       int    // REDIS_DB (номер БД, по умолчанию 0)

	// MinIO
	MinIOEndpoint        string // MINIO_ENDPOINT (хост:порт)
	MinIOAccessKey       string // MINIO_ACCESS_KEY
	MinIOSecretKey       string // MINIO_SECRET_KEY
	MinIOUseSSL          bool   // MINIO_USE_SSL
	MinIOBucketOriginals string // MINIO_BUCKET_ORIGINALS
	MinIOBucketVariants  string // MINIO_BUCKET_VARIANTS

	// Worker Pool
	WorkerCount int // WORKER_COUNT (количество воркеров)

	// Prometheus
	PrometheusPort int // PROMETHEUS_PORT

	// Параметры изображений (оставлены для гибкости)
	ImageThumbSize  int    // IMAGE_THUMB_SIZE (по умолчанию 300)
	ImageSmallSize  int    // IMAGE_SMALL_SIZE (480)
	ImageMediumSize int    // IMAGE_MEDIUM_SIZE (768)
	ImageLargeSize  int    // IMAGE_LARGE_SIZE (1200)
	ImageQuality    int    // IMAGE_QUALITY (80)
	ImageLibrary    string // IMAGE_LIBRARY (было bimg/imaging, теперь govips)
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	port, err := strconv.Atoi(getEnv("SERVER_PORT", "3000"))
	if err != nil {
		return nil, err
	}

	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "5432"))
	if err != nil {
		return nil, err
	}

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))

	workerCount, _ := strconv.Atoi(getEnv("WORKER_COUNT", "4"))
	prometheusPort, _ := strconv.Atoi(getEnv("PROMETHEUS_PORT", "9091"))

	return &Config{
		Port: port,
		Env:  getEnv("SERVER_ENV", "development"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     dbPort,
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "photohub"),

		JWTSecret: getEnv("JWT_SECRET", "your-secret-key"),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,

		MinIOEndpoint:        getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:       getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:       getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOUseSSL:          getEnvBool("MINIO_USE_SSL", false),
		MinIOBucketOriginals: getEnv("MINIO_BUCKET_ORIGINALS", "originals"),
		MinIOBucketVariants:  getEnv("MINIO_BUCKET_VARIANTS", "variants"),

		WorkerCount:    workerCount,
		PrometheusPort: prometheusPort,

		ImageThumbSize:  getEnvInt("IMAGE_THUMB_SIZE", 300),
		ImageSmallSize:  getEnvInt("IMAGE_SMALL_SIZE", 480),
		ImageMediumSize: getEnvInt("IMAGE_MEDIUM_SIZE", 768),
		ImageLargeSize:  getEnvInt("IMAGE_LARGE_SIZE", 1200),
		ImageQuality:    getEnvInt("IMAGE_QUALITY", 80),
		ImageLibrary:    "govips",
	}, nil
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	val := getEnv(key, "")
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}

func getEnvBool(key string, defaultVal bool) bool {
	val := strings.ToLower(getEnv(key, ""))
	if val == "" {
		return defaultVal
	}
	return val == "1" || val == "true" || val == "yes" || val == "y" || val == "on"
}
