package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port       int
	Env        string
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	JWTSecret  string

	ImageThumbSize  int    `mapstructure:"IMAGE_THUMB_SIZE" default:"300"`
	ImageSmallSize  int    `mapstructure:"IMAGE_SMALL_SIZE" default:"480"`
	ImageMediumSize int    `mapstructure:"IMAGE_MEDIUM_SIZE" default:"768"`
	ImageLargeSize  int    `mapstructure:"IMAGE_LARGE_SIZE" default:"1200"`
	ImagePipelineOn bool   `mapstructure:"IMAGE_PIPELINE_ON" default:"true"`
	WebPEnabled     bool   `mapstructure:"IMAGE_WEBP_ENABLED" default:"true"`
	ImageQuality    int    `mapstructure:"IMAGE_QUALITY" default:"80"`
	AsyncProcessing bool   `mapstructure:"IMAGE_ASYNC_PROCESSING" default:"true"`
	ImageLibrary    string `mapstructure:"IMAGE_LIBRARY" default:"bimg"`
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

	return &Config{
		Port:       port,
		Env:        getEnv("SERVER_ENV", "development"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     dbPort,
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "photohub"),
		JWTSecret:  getEnv("JWT_SECRET", "your-secret-key-change-in-production"),

		ImageThumbSize:  getEnvInt("IMAGE_THUMB_SIZE", 300),
		ImageSmallSize:  getEnvInt("IMAGE_SMALL_SIZE", 480),
		ImageMediumSize: getEnvInt("IMAGE_MEDIUM_SIZE", 768),
		ImageLargeSize:  getEnvInt("IMAGE_LARGE_SIZE", 1200),

		ImagePipelineOn: getEnvBool("IMAGE_PIPELINE_ON", false),
		WebPEnabled:     getEnvBool("IMAGE_WEBP_ENABLED", true),
		ImageQuality:    getEnvInt("IMAGE_QUALITY", 80),
		AsyncProcessing: getEnvBool("IMAGE_ASYNC_PROCESSING", false),

		ImageLibrary: getEnv("IMAGE_LIBRARY", "bimg"),
	}, nil
}

func getEnvBool(key string, defaultVal bool) bool {
	val := strings.ToLower(getEnv(key, ""))
	if val == "" {
		return defaultVal
	}
	return val == "1" || val == "true" || val == "yes" || val == "y" || val == "on"
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

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}
