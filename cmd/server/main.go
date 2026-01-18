package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/freshtea599/PhotoHubServer.git/internal/config"
	"github.com/freshtea599/PhotoHubServer.git/internal/http"
)

func init() {
	_ = godotenv.Load()
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Подключение к PostgreSQL
	dbURL := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)

	database, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Проверка соединения
	if err := database.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	log.Println("✅ Connected to PostgreSQL")

	// Создаём папку uploads если её нет
	if err := os.MkdirAll("uploads", 0755); err != nil {
		log.Fatalf("failed to create uploads directory: %v", err)
	}

	// Инициализируем HTTP сервер с БД
	srv := http.NewServer(cfg, database)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("🚀 Starting server on %s in %s mode", addr, cfg.Env)

	if err := srv.Start(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
