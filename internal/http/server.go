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

	jwtManager := auth.NewJWTManager(cfg.JWTSecret)

	userRepo := repository.NewUserRepository(db)
	photoRepo := repository.NewPhotoRepository(db)
	commentRepo := repository.NewCommentRepository(db)

	h := NewHandlers(cfg, jwtManager, userRepo, photoRepo, commentRepo)

	e.Use(CORSMiddleware)

	// ---------------- Public routes ----------------
	e.POST("/api/register", h.Register)
	e.POST("/api/login", h.Login)

	e.GET("/api/photos", h.ListPhotos)

	e.GET("/api/photos/:id/comments", h.GetComments)

	e.GET("/api/photos/:id", h.GetPhoto)

	e.Static("/uploads", "uploads")

	// ---------------- Protected routes ----------------
	api := e.Group("/api")
	api.Use(JWTMiddleware(jwtManager))

	api.GET("/auth/me", h.GetMe)

	api.GET("/me/photos", h.ListMyPhotos)

	api.GET("/photos/me", h.ListMyPhotos)
	api.GET("/photos/mine", h.ListMyPhotos)

	api.POST("/photos/upload", h.UploadPhoto)
	api.PUT("/photos/:id", h.UpdatePhoto)
	api.DELETE("/photos/:id", h.DeletePhoto)

	api.POST("/photos/:id/like", h.LikePhoto)
	api.DELETE("/photos/:id/like", h.UnlikePhoto)
	api.GET("/photos/:id/like", h.IsPhotoLiked)

	api.POST("/photos/:id/comments", h.CreateComment)
	api.POST("/comments/:comment_id/like", h.LikeComment)
	api.DELETE("/comments/:comment_id/like", h.UnlikeComment)
	api.POST("/comments/:comment_id/report", h.ReportComment)
	api.DELETE("/comments/:comment_id", h.DeleteComment)

	// ---------------- Admin routes ----------------
	admin := api.Group("/admin")

	admin.GET("/photos/pending", h.GetPendingPhotos)
	admin.POST("/photos/:id/approve", h.ApprovePhoto)
	admin.POST("/photos/:id/reject", h.RejectPhoto)

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
