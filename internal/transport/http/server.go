// backend/internal/transport/http/server.go
package http

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"

	"github.com/freshtea599/PhotoHubServer.git/internal/auth"
	"github.com/freshtea599/PhotoHubServer.git/internal/config"
	"github.com/freshtea599/PhotoHubServer.git/internal/repository"
	"github.com/freshtea599/PhotoHubServer.git/internal/usecase"
)

type Server struct {
	echo           *echo.Echo
	cfg            *config.Config
	imageProcessor *usecase.ImageProcessor
	photoRepo      *repository.PostgresPhotoRepo
}

func NewServer(
	cfg *config.Config,
	jwtManager *auth.JWTManager,
	userRepo *repository.PostgresUserRepo,
	photoRepo *repository.PostgresPhotoRepo,
	minioRepo *repository.MinioRepo,
	redisClient *redis.Client,
	minioClient *minio.Client,
	imgProc *usecase.ImageProcessor,
) *Server {
	e := echo.New()

	// CORS middleware (как раньше)
	e.Use(CORSMiddleware)

	// Инициализация обработчиков
	h := NewHandlers(cfg, jwtManager, userRepo, photoRepo, minioRepo, imgProc)

	// Публичные маршруты
	e.POST("/api/register", h.Register)
	e.POST("/api/login", h.Login)
	e.GET("/api/photos", h.ListPhotos)
	e.GET("/api/photos/:id/variant", h.GetImageVariant)

	// Защищённые маршруты (JWT)
	api := e.Group("/api")
	api.Use(JWTMiddleware(jwtManager))
	api.POST("/upload", h.UploadPhoto) // POST /api/upload
	api.GET("/photos/mine", h.GetMyPhotos)

	// Статическая раздача оригиналов (если нужно, через Echo)
	// В проде это делает Nginx, но для отладки оставим
	e.Static("/media", "uploads") // локальная папка (можно заменить на MinIO-прокси, но пока так)

	return &Server{
		echo:           e,
		cfg:            cfg,
		imageProcessor: imgProc,
		photoRepo:      photoRepo,
	}
}

func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}
