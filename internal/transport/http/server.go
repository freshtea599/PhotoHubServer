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
	e.Use(CORSMiddleware)
	h := NewHandlers(cfg, jwtManager, userRepo, photoRepo, minioRepo, imgProc)

	// Публичные
	e.POST("/api/register", h.Register)
	e.POST("/api/login", h.Login)
	e.GET("/api/photos", h.ListPhotos)
	e.GET("/api/photos/:id/variant", h.GetImageVariant)

	// Защищённые
	api := e.Group("/api")
	api.Use(JWTMiddleware(jwtManager))
	api.GET("/auth/me", h.GetMe)
	api.POST("/upload", h.UploadPhoto)
	api.GET("/photos/mine", h.GetMyPhotos)
	api.PUT("/photos/:id", h.UpdatePhoto)
	api.DELETE("/photos/:id", h.DeletePhoto)
	api.POST("/photos/:id/like", h.LikePhoto)
	api.DELETE("/photos/:id/like", h.UnlikePhoto)
	api.GET("/photos/:id/like", h.IsPhotoLiked)
	api.GET("/photos/:id/comments", h.GetComments)
	api.POST("/photos/:id/comments", h.CreateComment)
	api.POST("/comments/:id/like", h.LikeComment)
	api.DELETE("/comments/:id/like", h.UnlikeComment)
	api.POST("/comments/:id/report", h.ReportComment)

	// Админские (проверку is_admin добавить позже)
	admin := e.Group("/admin")
	admin.Use(JWTMiddleware(jwtManager))
	admin.GET("/photos/pending", h.GetPendingPhotos)
	admin.POST("/photos/:id/approve", h.ApprovePhoto)
	admin.POST("/photos/:id/reject", h.RejectPhoto)

	e.Static("/media", "uploads")

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
