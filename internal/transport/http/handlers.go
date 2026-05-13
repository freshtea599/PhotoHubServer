// backend/internal/transport/http/handlers.go
package http

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/freshtea599/PhotoHubServer.git/internal/auth"
	"github.com/freshtea599/PhotoHubServer.git/internal/config"
	"github.com/freshtea599/PhotoHubServer.git/internal/domain"
	"github.com/freshtea599/PhotoHubServer.git/internal/repository"
	"github.com/freshtea599/PhotoHubServer.git/internal/usecase"
)

// Handlers содержит все обработчики HTTP.
type Handlers struct {
	cfg            *config.Config
	jwtManager     *auth.JWTManager
	userRepo       *repository.PostgresUserRepo
	photoRepo      *repository.PostgresPhotoRepo
	minioRepo      *repository.MinioRepo
	imageProcessor *usecase.ImageProcessor // может быть nil на начальном этапе
}

// NewHandlers создаёт новый экземпляр обработчиков.
func NewHandlers(
	cfg *config.Config,
	jwtManager *auth.JWTManager,
	userRepo *repository.PostgresUserRepo,
	photoRepo *repository.PostgresPhotoRepo,
	minioRepo *repository.MinioRepo,
	imgProc *usecase.ImageProcessor,
) *Handlers {
	return &Handlers{
		cfg:            cfg,
		jwtManager:     jwtManager,
		userRepo:       userRepo,
		photoRepo:      photoRepo,
		minioRepo:      minioRepo,
		imageProcessor: imgProc,
	}
}

// ---------- helpers ----------
func getUserID(c echo.Context) (int64, bool) {
	v := c.Get("user_id")
	if v == nil {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok && id > 0
}

// ---------- auth (оставлены минимально, если нужна регистрация) ----------
func (h *Handlers) Register(c echo.Context) error {
	// (реализация как раньше, но с новыми типами)
	var req domain.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Email == "" || req.Password == "" || req.Username == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email, password, and username are required"})
	}
	if len(req.Password) < 6 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "password must be at least 6 characters"})
	}
	user, err := h.userRepo.Create(req.Email, req.Password, req.Username)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email already exists"})
	}
	token, err := h.jwtManager.GenerateToken(user.ID, user.Email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
	}
	return c.JSON(http.StatusCreated, domain.AuthResponse{Token: token, User: user})
}

func (h *Handlers) Login(c echo.Context) error {
	var req domain.LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	user, err := h.userRepo.CheckPassword(req.Email, req.Password)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
	}
	token, err := h.jwtManager.GenerateToken(user.ID, user.Email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
	}
	return c.JSON(http.StatusOK, domain.AuthResponse{Token: token, User: user})
}

// ---------- photos ----------

// UploadPhoto загружает новый файл в MinIO и сохраняет метаданные.
func (h *Handlers) UploadPhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	file, err := c.FormFile("photo")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "photo file is required"})
	}

	// Ограничение размера (50 МБ)
	if file.Size > 50*1024*1024 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "photo size must not exceed 50MB"})
	}

	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}
	mimeType := file.Header.Get("Content-Type")
	if !allowedTypes[mimeType] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid image format. allowed: jpeg, png, webp"})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to open file"})
	}
	defer src.Close()

	// Читаем содержимое для вычисления хэша и загрузки в MinIO
	fileBytes, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read file"})
	}

	// Генерируем уникальное имя (UUID + расширение)
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		switch mimeType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		}
	}
	objectKey := uuid.New().String() + ext

	// Загрузка в MinIO (бакет originals)
	err = h.minioRepo.PutOriginal(c.Request().Context(), objectKey, bytesReader(fileBytes), file.Size, mimeType)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to store file"})
	}

	// Вычисляем Content Hash (SHA-256)
	hash := fmt.Sprintf("%x", sha256.Sum256(fileBytes))

	// TODO: вычислить BlurHash и размеры (пока заглушка)
	blurHash := "L6PZfSi_.AyE_3t7t7R**0o#DgR4" // пример, позже заменим
	width, height := 0, 0                      // позже получим реальные

	// Сохраняем метаданные в БД
	photo := &domain.Photo{
		UserID:      userID,
		URL:         "/media/originals/" + objectKey,
		FilePath:    objectKey,
		FileSize:    sql.NullInt64{Int64: file.Size, Valid: true},
		MimeType:    mimeType,
		Description: c.FormValue("description"),
		IsPublic:    c.FormValue("is_public") == "true",
		BlurHash:    blurHash,
		ContentHash: hash,
		Width:       width,
		Height:      height,
	}

	savedPhoto, err := h.photoRepo.Create(photo)
	if err != nil {
		// если не удалось сохранить в БД, удаляем из MinIO
		_ = h.minioRepo.DeleteOriginal(c.Request().Context(), objectKey)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save metadata"})
	}

	return c.JSON(http.StatusCreated, savedPhoto)
}

// ListPhotos возвращает публичные фото с пагинацией.
func (h *Handlers) ListPhotos(c echo.Context) error {
	limit := 20
	offset := 0
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := c.QueryParam("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	photos, err := h.photoRepo.ListPublic(limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch photos"})
	}
	return c.JSON(http.StatusOK, photos)
}

// GetImageVariant – JIT-эндпоинт: возвращает оптимизированное изображение.
func (h *Handlers) GetImageVariant(c echo.Context) error {
	photoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || photoID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}

	width, _ := strconv.Atoi(c.QueryParam("width"))
	format := c.QueryParam("format") // "webp", "avif", "jpeg"
	if format == "" {
		format = "jpeg"
	}
	quality, _ := strconv.Atoi(c.QueryParam("q"))
	if quality < 1 || quality > 100 {
		quality = 80
	}

	// Если ImageProcessor ещё не подключён, возвращаем ошибку (или оригинал)
	if h.imageProcessor == nil {
		// Заглушка: отдаём оригинал из MinIO
		photo, err := h.photoRepo.GetByID(photoID)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "photo not found"})
		}
		reader, err := h.minioRepo.GetOriginal(c.Request().Context(), photo.FilePath)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "original file not found"})
		}
		defer reader.Close()
		data, _ := io.ReadAll(reader)
		return c.Blob(http.StatusOK, photo.MimeType, data)
	}

	// Основной сценарий – JIT-трансформация
	sizeName := fmt.Sprintf("%dw", width) // например "400w"
	data, contentType, err := h.imageProcessor.GetVariant(
		c.Request().Context(),
		photoID,
		sizeName,
		width,
		format,
		quality,
	)
	if err != nil {
		log.Printf("JIT variant error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate variant"})
	}

	return c.Blob(http.StatusOK, contentType, data)
}

// bytesReader – вспомогательная функция.
func bytesReader(data []byte) *bytes.Reader {
	return bytes.NewReader(data)
}

// GetMyPhotos возвращает фото текущего пользователя
func (h *Handlers) GetMyPhotos(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	limit := 50
	offset := 0
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := c.QueryParam("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	photos, err := h.photoRepo.ListByUser(userID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch photos"})
	}
	return c.JSON(http.StatusOK, photos)
}
