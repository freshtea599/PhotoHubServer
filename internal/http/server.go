package http

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/freshtea599/PhotoHubServer.git/internal/config"
	"github.com/freshtea599/PhotoHubServer.git/internal/models"
	"github.com/freshtea599/PhotoHubServer.git/internal/repository"
)

type Server struct {
	echo   *echo.Echo
	cfg    *config.Config
	photos *repository.PhotoRepository
	users  *repository.UserRepository
}

func NewServer(cfg *config.Config) *Server {
	e := echo.New()

	srv := &Server{
		echo:   e,
		cfg:    cfg,
		photos: repository.NewPhotoRepository(),
		users:  repository.NewUserRepository(),
	}

	// Публичные маршруты
	e.GET("/ping", srv.handlePing)
	e.POST("/api/users", srv.handleCreateUser)
	e.GET("/api/users/:id", srv.handleGetUserByID)

	// Защищённые маршруты (требуют авторизации)
	authGroup := e.Group("")
	authGroup.Use(srv.CurrentUserMiddleware)

	authGroup.GET("/api/photos", srv.handleListPhotos)
	authGroup.POST("/api/photos", srv.handleCreatePhoto)
	authGroup.GET("/api/photos/:id", srv.handleGetPhotoByID)
	authGroup.DELETE("/api/photos/:id", srv.handleDeletePhoto)

	return srv
}

func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}

// ==================== MIDDLEWARE ====================

func (s *Server) CurrentUserMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		idStr := c.Request().Header.Get("X-User-ID")
		if idStr == "" {
			return c.JSON(http.StatusUnauthorized, map[string]any{
				"error": "missing X-User-ID header",
			})
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusUnauthorized, map[string]any{
				"error": "invalid X-User-ID",
			})
		}

		user, err := s.users.GetByID(id)
		if err != nil {
			if err == repository.ErrUserNotFound {
				return c.JSON(http.StatusUnauthorized, map[string]any{
					"error": "user not found",
				})
			}
			return c.JSON(http.StatusInternalServerError, map[string]any{
				"error": "failed to load user",
			})
		}

		c.Set("currentUser", user)
		return next(c)
	}
}

func getCurrentUser(c echo.Context) (models.User, bool) {
	v := c.Get("currentUser")
	if v == nil {
		return models.User{}, false
	}
	user, ok := v.(models.User)
	return user, ok
}

// ==================== HANDLERS ====================

func (s *Server) handlePing(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"message": "pong",
		"env":     s.cfg.Env,
	})
}

// ====== USERS ======

type createUserRequest struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}

func (s *Server) handleCreateUser(c echo.Context) error {
	var req createUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": "invalid request body",
		})
	}

	if req.Username == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": "username is required",
		})
	}

	user := models.User{
		Username: req.Username,
		IsAdmin:  req.IsAdmin,
	}

	created := s.users.Create(user)
	return c.JSON(http.StatusCreated, created)
}

func (s *Server) handleGetUserByID(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": "invalid id",
		})
	}

	user, err := s.users.GetByID(id)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return c.JSON(http.StatusNotFound, map[string]any{
				"error": "user not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error": "failed to get user",
		})
	}

	return c.JSON(http.StatusOK, user)
}

// ====== PHOTOS ======

type createPhotoRequest struct {
	URL         string `json:"url"`
	UserID      int64  `json:"user_id"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

func (s *Server) handleCreatePhoto(c echo.Context) error {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"error": "no current user",
		})
	}

	var req createPhotoRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": "invalid request body",
		})
	}

	userID := currentUser.ID
	if req.UserID != 0 {
		if req.UserID != currentUser.ID && !currentUser.IsAdmin {
			return c.JSON(http.StatusForbidden, map[string]any{
				"error": "cannot create photo for another user",
			})
		}
		userID = req.UserID
	}

	if req.URL == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": "url is required",
		})
	}

	photo := models.Photo{
		URL:         req.URL,
		UserID:      userID,
		Description: req.Description,
		IsPublic:    req.IsPublic,
		LikesCount:  0,
	}

	created := s.photos.Create(photo)
	return c.JSON(http.StatusCreated, created)
}

func (s *Server) handleListPhotos(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit := 20
	offset := 0

	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	if offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}

	items := s.photos.List(limit, offset)

	return c.JSON(http.StatusOK, map[string]any{
		"limit":  limit,
		"offset": offset,
		"items":  items,
	})
}

func (s *Server) handleGetPhotoByID(c echo.Context) error {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"error": "no current user",
		})
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": "invalid id",
		})
	}

	photo, err := s.photos.GetByID(id)
	if err != nil {
		if err == repository.ErrPhotoNotFound {
			return c.JSON(http.StatusNotFound, map[string]any{
				"error": "photo not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error": "failed to get photo",
		})
	}

	if !photo.IsPublic && photo.UserID != currentUser.ID && !currentUser.IsAdmin {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": "no access to this photo",
		})
	}

	return c.JSON(http.StatusOK, photo)
}

func (s *Server) handleDeletePhoto(c echo.Context) error {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"error": "no current user",
		})
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": "invalid id",
		})
	}

	photo, err := s.photos.GetByID(id)
	if err != nil {
		if err == repository.ErrPhotoNotFound {
			return c.JSON(http.StatusNotFound, map[string]any{
				"error": "photo not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error": "failed to get photo",
		})
	}

	if photo.UserID != currentUser.ID && !currentUser.IsAdmin {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": "cannot delete this photo",
		})
	}

	if err := s.photos.Delete(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error": "failed to delete photo",
		})
	}

	return c.NoContent(http.StatusNoContent)
}
