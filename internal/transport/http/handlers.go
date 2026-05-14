package http

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
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

type Handlers struct {
	cfg            *config.Config
	jwtManager     *auth.JWTManager
	userRepo       *repository.PostgresUserRepo
	photoRepo      *repository.PostgresPhotoRepo
	minioRepo      *repository.MinioRepo
	imageProcessor *usecase.ImageProcessor
}

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

func getUserID(c echo.Context) (int64, bool) {
	v := c.Get("user_id")
	if v == nil {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok && id > 0
}

// ---------- Auth ----------
func (h *Handlers) Register(c echo.Context) error {
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

func (h *Handlers) GetMe(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	return c.JSON(http.StatusOK, user)
}

// ---------- Photos ----------
func (h *Handlers) UploadPhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	file, err := c.FormFile("photo")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "photo file is required"})
	}
	if file.Size > 50*1024*1024 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "photo size must not exceed 50MB"})
	}
	allowedTypes := map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}
	mimeType := file.Header.Get("Content-Type")
	if !allowedTypes[mimeType] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid image format. allowed: jpeg, png, webp"})
	}
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to open file"})
	}
	defer src.Close()
	fileBytes, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read file"})
	}

	// Попытка вычислить размеры и blurhash (если не получится – игнорируем)
	var width, height int
	var blurHashStr string
	img, _, decodeErr := image.Decode(bytes.NewReader(fileBytes))
	if decodeErr == nil {
		bounds := img.Bounds()
		width, height = bounds.Dx(), bounds.Dy()
		// Если пакет blurhash доступен, можно закодировать, но не критично
		// blurHashStr, _ = blurhash.Encode(4, 3, img)
		// Пока оставим пустым, чтобы избежать лишних зависимостей
	} else {
		log.Printf("Warning: failed to decode image for dimensions/blurhash: %v", decodeErr)
	}

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
	err = h.minioRepo.PutOriginal(c.Request().Context(), objectKey, bytes.NewReader(fileBytes), file.Size, mimeType)
	if err != nil {
		log.Printf("MinIO put error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to store file"})
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(fileBytes))

	photo := &domain.Photo{
		UserID:      userID,
		URL:         "/media/originals/" + objectKey,
		FilePath:    objectKey,
		FileSize:    sql.NullInt64{Int64: file.Size, Valid: true},
		MimeType:    mimeType,
		Description: c.FormValue("description"),
		IsPublic:    c.FormValue("is_public") == "true",
		BlurHash:    blurHashStr,
		ContentHash: hash,
		Width:       width,
		Height:      height,
	}
	savedPhoto, err := h.photoRepo.Create(photo)
	if err != nil {
		_ = h.minioRepo.DeleteOriginal(c.Request().Context(), objectKey)
		log.Printf("DB create error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save metadata"})
	}
	return c.JSON(http.StatusCreated, savedPhoto)
}
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

func (h *Handlers) UpdatePhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	photoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}
	var req domain.UpdatePhotoRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	photo, err := h.photoRepo.GetByID(photoID)
	if err != nil || photo.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "access denied"})
	}
	updated, err := h.photoRepo.Update(photoID, req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update"})
	}
	return c.JSON(http.StatusOK, updated)
}

func (h *Handlers) DeletePhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	photoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}
	photo, err := h.photoRepo.GetByID(photoID)
	if err != nil || photo.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "access denied"})
	}
	// удаляем из MinIO
	_ = h.minioRepo.DeleteOriginal(c.Request().Context(), photo.FilePath)
	if err := h.minioRepo.DeleteVariants(c.Request().Context(), photoID); err != nil {
		log.Printf("Failed to delete variants: %v", err)
	}
	// удаляем из БД
	if err := h.photoRepo.Delete(photoID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete"})
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handlers) GetImageVariant(c echo.Context) error {
	photoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || photoID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}

	width, _ := strconv.Atoi(c.QueryParam("width"))
	quality, _ := strconv.Atoi(c.QueryParam("q"))
	if quality < 1 || quality > 100 {
		quality = 80
	}

	format := c.QueryParam("format")
	if format == "" {
		accept := c.Request().Header.Get("Accept")
		switch {
		case strings.Contains(accept, "avif"):
			format = "avif"
		case strings.Contains(accept, "webp"):
			format = "webp"
		default:
			format = "jpeg"
		}
	}

	// Получаем фото один раз
	photo, err := h.photoRepo.GetByID(photoID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "photo not found"})
	}

	// Если imageProcessor не инициализирован, отдаём оригинал
	if h.imageProcessor == nil {
		reader, err := h.minioRepo.GetOriginal(c.Request().Context(), photo.FilePath)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "original file not found"})
		}
		defer reader.Close()
		data, _ := io.ReadAll(reader)
		return c.Blob(http.StatusOK, photo.MimeType, data)
	}

	// Запрос варианта
	data, contentType, err := h.imageProcessor.GetVariant(
		c.Request().Context(),
		photoID,
		fmt.Sprintf("%dw", width),
		width,
		format,
		quality,
	)
	if err != nil {
		log.Printf("JIT variant error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate variant"})
	}

	// Заголовки кэширования и валидации
	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if photo.ContentHash != "" {
		c.Response().Header().Set("X-Content-Hash", photo.ContentHash)
	}
	return c.Blob(http.StatusOK, contentType, data)
}

// ---------- Likes for photos ----------
func (h *Handlers) LikePhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	photoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}
	if err := h.photoRepo.LikePhoto(photoID, userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to like"})
	}
	photo, _ := h.photoRepo.GetByID(photoID)
	return c.JSON(http.StatusOK, photo)
}

func (h *Handlers) UnlikePhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	photoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}
	if err := h.photoRepo.UnlikePhoto(photoID, userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to unlike"})
	}
	photo, _ := h.photoRepo.GetByID(photoID)
	return c.JSON(http.StatusOK, photo)
}

func (h *Handlers) IsPhotoLiked(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	photoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}
	liked, err := h.photoRepo.IsPhotoLiked(photoID, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]bool{"liked": liked})
}

// ---------- Comments ----------
func (h *Handlers) GetComments(c echo.Context) error {
	photoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}
	comments, err := h.photoRepo.GetComments(photoID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch comments"})
	}
	return c.JSON(http.StatusOK, comments)
}

func (h *Handlers) CreateComment(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	photoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := c.Bind(&req); err != nil || req.Text == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "text required"})
	}
	commentID, err := h.photoRepo.CreateComment(photoID, userID, req.Text)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create comment"})
	}
	comments, err := h.photoRepo.GetComments(photoID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch comments"})
	}
	for _, comm := range comments { // ← переименовано с "c" на "comm"
		if comm.ID == commentID {
			return c.JSON(http.StatusCreated, comm)
		}
	}
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "comment not found"})
}

// ---------- Comment likes ----------
func (h *Handlers) LikeComment(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid comment id"})
	}
	if err := h.photoRepo.LikeComment(commentID, userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to like comment"})
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handlers) UnlikeComment(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid comment id"})
	}
	if err := h.photoRepo.UnlikeComment(commentID, userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to unlike comment"})
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handlers) ReportComment(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid comment id"})
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.Bind(&req); err != nil || req.Reason == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "reason required"})
	}
	if err := h.photoRepo.ReportComment(commentID, userID, req.Reason); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to report comment"})
	}
	return c.NoContent(http.StatusOK)
}

// ---------- Admin ----------
func (h *Handlers) GetPendingPhotos(c echo.Context) error {
	photos, err := h.photoRepo.GetPendingPhotos()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch pending photos"})
	}
	return c.JSON(http.StatusOK, photos)
}

func (h *Handlers) ApprovePhoto(c echo.Context) error {
	photoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	if err := h.photoRepo.ApprovePhoto(photoID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to approve"})
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handlers) RejectPhoto(c echo.Context) error {
	photoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	if err := h.photoRepo.RejectPhoto(photoID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to reject"})
	}
	return c.NoContent(http.StatusOK)
}

// Helper
func bytesReader(data []byte) *bytes.Reader {
	return bytes.NewReader(data)
}
