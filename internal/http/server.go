package http

import (
	"database/sql"

	"github.com/labstack/echo/v4"

	"github.com/freshtea599/PhotoHubServer.git/internal/auth"
	"github.com/freshtea599/PhotoHubServer.git/internal/config"
	"github.com/freshtea599/PhotoHubServer.git/internal/repository"
)

type Server struct {
	echo *echo.Echo
	cfg  *config.Config
}

func NewServer(cfg *config.Config, db *sql.DB) *Server {
	e := echo.New()

	// Core deps
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)

	// Repositories
	userRepo := repository.NewUserRepository(db)
	photoRepo := repository.NewPhotoRepository(db)
	commentRepo := repository.NewCommentRepository(db)

	// Handlers
	h := NewHandlers(cfg, jwtManager, userRepo, photoRepo, commentRepo)

	// Middlewares
	e.Use(CORSMiddleware)

	// ---------------- Public routes ----------------
	e.POST("/api/register", h.Register)
	e.POST("/api/login", h.Login)

	// Публичная галерея (только одобренные — если так сделано в repo)
	e.GET("/api/photos", h.ListPhotos)

	// Комментарии к фото можно читать публично
	e.GET("/api/photos/:id/comments", h.GetComments)

	// ВАЖНО: /api/photos/:id (параметр) регистрируем ПОСЛЕ всех статических путей /api/photos/...
	e.GET("/api/photos/:id", h.GetPhoto)

	// Static uploads
	e.Static("/uploads", "uploads")

	// ---------------- Protected routes ----------------
	api := e.Group("/api")
	api.Use(JWTMiddleware(jwtManager))

	// Auth
	api.GET("/auth/me", h.GetMe)

	// My library (основной путь)
	api.GET("/me/photos", h.ListMyPhotos)

	// Алиасы под частые варианты во фронте:
	// чтобы /api/photos/me или /api/photos/mine не попадали в /api/photos/:id
	api.GET("/photos/me", h.ListMyPhotos)
	api.GET("/photos/mine", h.ListMyPhotos)

	// Photo CRUD
	api.POST("/photos/upload", h.UploadPhoto)
	api.PUT("/photos/:id", h.UpdatePhoto)
	api.DELETE("/photos/:id", h.DeletePhoto)

	// Photo likes
	api.POST("/photos/:id/like", h.LikePhoto)
	api.DELETE("/photos/:id/like", h.UnlikePhoto)
	api.GET("/photos/:id/like", h.IsPhotoLiked)

	// Comments
	api.POST("/photos/:id/comments", h.CreateComment)
	api.POST("/comments/:comment_id/like", h.LikeComment)
	api.DELETE("/comments/:comment_id/like", h.UnlikeComment)
	api.POST("/comments/:comment_id/report", h.ReportComment)
	api.DELETE("/comments/:comment_id", h.DeleteComment)

	// ---------------- Admin routes ----------------
	// Внутри handlers идёт проверка user.IsAdmin
	admin := api.Group("/admin")

	// Photos moderation
	admin.GET("/photos/pending", h.GetPendingPhotos)
	admin.POST("/photos/:id/approve", h.ApprovePhoto)
	admin.POST("/photos/:id/reject", h.RejectPhoto)

	// Comment reports moderation
	admin.GET("/comment-reports", h.GetCommentReports)
	admin.POST("/comment-reports/:report_id/resolve", h.ResolveCommentReport)

	return &Server{
		echo: e,
		cfg:  cfg,
	}
}

func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}
