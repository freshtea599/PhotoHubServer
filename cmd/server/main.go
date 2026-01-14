package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"

	"github.com/freshtea599/PhotoHubServer.git/internal/config"
	"github.com/freshtea599/PhotoHubServer.git/internal/http"
)

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

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	log.Println("Connected to PostgreSQL")

	srv := http.NewServer(cfg)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting server on %s in %s mode", addr, cfg.Env)

	if err := srv.Start(addr); err != nil {
		log.Fatal(err)
	}
}
