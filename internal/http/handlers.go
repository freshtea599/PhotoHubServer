package http

import (
	"database/sql"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/freshtea599/PhotoHubServer.git/internal/auth"
	"github.com/freshtea599/PhotoHubServer.git/internal/models"
	"github.com/freshtea599/PhotoHubServer.git/internal/repository"
)

type Handlers struct {
	jwtManager  *auth.JWTManager
	userRepo    *repository.UserRepository
	photoRepo   *repository.PhotoRepository
	commentRepo *repository.CommentRepository
}

func NewHandlers(
	jwtManager *auth.JWTManager,
	userRepo *repository.UserRepository,
	photoRepo *repository.PhotoRepository,
	commentRepo *repository.CommentRepository,
) *Handlers {
	return &Handlers{
		jwtManager:  jwtManager,
		userRepo:    userRepo,
		photoRepo:   photoRepo,
		commentRepo: commentRepo,
	}
}

// ---------- helpers ----------

func getUserID(c echo.Context) (int64, bool) {
	v := c.Get("user_id")
	if v == nil {
		return 0, false
	}
	id, ok := v.(int64)
	if !ok || id <= 0 {
		return 0, false
	}
	return id, true
}

func parseIDParam(c echo.Context, name string) (int64, error) {
	return strconv.ParseInt(c.Param(name), 10, 64)
}

func isAdminUser(u *models.User) bool {
	if u == nil {
		return false
	}
	return u.IsAdmin
}

// ---------- auth ----------

// Register регистрирует нового пользователя
func (h *Handlers) Register(c echo.Context) error {
	var req models.RegisterRequest
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
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email already exists or registration failed"})
	}

	token, err := h.jwtManager.GenerateToken(user.ID, user.Email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
	}

	return c.JSON(http.StatusCreated, models.AuthResponse{
		Token: token,
		User:  user,
	})
}

// Login входит в аккаунт
func (h *Handlers) Login(c echo.Context) error {
	var req models.LoginRequest
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

	return c.JSON(http.StatusOK, models.AuthResponse{
		Token: token,
		User:  user,
	})
}

// GetMe получает информацию текущего пользователя
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

// ---------- photos ----------

// UploadPhoto загружает новое фото
func (h *Handlers) UploadPhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	file, err := c.FormFile("photo")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "photo file is required"})
	}

	// max 10MB
	if file.Size > 10*1024*1024 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "photo size must not exceed 10MB"})
	}

	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	mimeType := file.Header.Get("Content-Type")
	if !allowedTypes[mimeType] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid image format. allowed: jpeg, png, gif, webp"})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to open file"})
	}
	defer src.Close()

	_ = os.MkdirAll("uploads", 0755)

	fileName := uuid.New().String() + filepath.Ext(file.Filename)
	filePath := filepath.Join("uploads", fileName)

	dst, err := os.Create(filePath)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save file"})
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(filePath)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to copy file"})
	}

	description := c.FormValue("description")

	// поддержка обоих вариантов: is_public и ispublic
	isPublic := false
	if c.FormValue("is_public") == "true" || c.FormValue("ispublic") == "true" {
		isPublic = true
	}

	photo := &models.Photo{
		UserID:      userID,
		URL:         "/" + filePath,
		FilePath:    filePath,
		FileSize:    sql.NullInt64{Int64: file.Size, Valid: true},
		MimeType:    mimeType,
		Description: description,
		IsPublic:    isPublic,
	}

	savedPhoto, err := h.photoRepo.Create(photo)
	if err != nil {
		_ = os.Remove(filePath)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save photo to database"})
	}

	return c.JSON(http.StatusCreated, savedPhoto)
}

// ListPhotos получает публичные фото (в твоей логике — только одобренные, если так сделано в repo)
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

// ListMyPhotos получает все фото текущего пользователя (включая приватные)
func (h *Handlers) ListMyPhotos(c echo.Context) error {
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
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch my photos"})
	}

	return c.JSON(http.StatusOK, photos)
}

// GetPhoto получает фото по ID
func (h *Handlers) GetPhoto(c echo.Context) error {
	id, err := parseIDParam(c, "id")
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}

	photo, err := h.photoRepo.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "photo not found"})
	}

	return c.JSON(http.StatusOK, photo)
}

// UpdatePhoto обновляет информацию фото
func (h *Handlers) UpdatePhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	id, err := parseIDParam(c, "id")
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}

	photo, err := h.photoRepo.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "photo not found"})
	}

	if photo.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "you can only update your own photos"})
	}

	var req models.UpdatePhotoRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	updatedPhoto, err := h.photoRepo.Update(id, &req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update photo"})
	}

	return c.JSON(http.StatusOK, updatedPhoto)
}

// DeletePhoto удаляет фото
func (h *Handlers) DeletePhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	id, err := parseIDParam(c, "id")
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}

	photo, err := h.photoRepo.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "photo not found"})
	}

	if photo.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "you can only delete your own photos"})
	}

	// удаляем файл (не критично, если уже нет)
	if err := os.Remove(photo.FilePath); err != nil && !os.IsNotExist(err) {
		// можно логировать, но не валим запрос
	}

	if err := h.photoRepo.Delete(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete photo"})
	}

	return c.NoContent(http.StatusNoContent)
}

// ---------- photo likes ----------

// LikePhoto ставит лайк фото
func (h *Handlers) LikePhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	id, err := parseIDParam(c, "id")
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}

	if err := h.photoRepo.LikePhoto(id, userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to like photo"})
	}

	photo, err := h.photoRepo.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "photo not found"})
	}

	return c.JSON(http.StatusOK, photo)
}

// UnlikePhoto удаляет лайк фото
func (h *Handlers) UnlikePhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	id, err := parseIDParam(c, "id")
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}

	if err := h.photoRepo.UnlikePhoto(id, userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to unlike photo"})
	}

	photo, err := h.photoRepo.GetByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "photo not found"})
	}

	return c.JSON(http.StatusOK, photo)
}

// IsPhotoLiked проверяет, лайкнул ли юзер фото
func (h *Handlers) IsPhotoLiked(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	id, err := parseIDParam(c, "id")
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}

	liked, err := h.photoRepo.IsPhotoLikedByUser(id, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check like status"})
	}

	return c.JSON(http.StatusOK, map[string]bool{"liked": liked})
}

// ---------- comments ----------

// GetComments получает комментарии фото (доступно публично, user_id может отсутствовать)
func (h *Handlers) GetComments(c echo.Context) error {
	photoID, err := parseIDParam(c, "id")
	if err != nil || photoID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}

	currentUserID, _ := getUserID(c) // если не авторизован, будет 0

	comments, err := h.commentRepo.GetCommentsByPhoto(photoID, currentUserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch comments"})
	}
	if comments == nil {
		comments = []*models.Comment{}
	}

	return c.JSON(http.StatusOK, comments)
}

// CreateComment создаёт комментарий
func (h *Handlers) CreateComment(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	photoID, err := parseIDParam(c, "id")
	if err != nil || photoID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}

	var req models.CreateCommentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Text == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "comment text is required"})
	}

	comment, err := h.commentRepo.CreateComment(photoID, userID, req.Text)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create comment"})
	}

	return c.JSON(http.StatusCreated, comment)
}

// LikeComment ставит лайк комментарию
func (h *Handlers) LikeComment(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	commentID, err := parseIDParam(c, "comment_id")
	if err != nil || commentID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid comment id"})
	}

	if err := h.commentRepo.LikeComment(commentID, userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to like comment"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "comment liked"})
}

// UnlikeComment удаляет лайк комментария
func (h *Handlers) UnlikeComment(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	commentID, err := parseIDParam(c, "comment_id")
	if err != nil || commentID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid comment id"})
	}

	if err := h.commentRepo.UnlikeComment(commentID, userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to unlike comment"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "comment unliked"})
}

// ReportComment жалуется на комментарий (ВАЖНО: тут userID используется -> ошибки unused не будет)
func (h *Handlers) ReportComment(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	commentID, err := parseIDParam(c, "comment_id")
	if err != nil || commentID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid comment id"})
	}

	var req models.ReportCommentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Reason == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "reason is required"})
	}

	// (опционально) проверить что коммент существует — чтобы не плодить мусор
	if _, err := h.commentRepo.GetCommentByID(commentID); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "comment not found"})
	}

	if err := h.commentRepo.ReportComment(commentID, userID, req.Reason); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to report comment"})
	}

	return c.JSON(http.StatusCreated, map[string]string{"message": "report sent"})
}

// DeleteComment удаляет комментарий (автор или админ) (ВАЖНО: userID используется -> ошибки unused не будет)
func (h *Handlers) DeleteComment(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	commentID, err := parseIDParam(c, "comment_id")
	if err != nil || commentID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid comment id"})
	}

	comment, err := h.commentRepo.GetCommentByID(commentID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "comment not found"})
	}

	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	if comment.UserID != userID && !isAdminUser(user) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "you can only delete your own comments"})
	}

	if err := h.commentRepo.DeleteComment(commentID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete comment"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "comment deleted"})
}

// ---------- admin  ----------

func (h *Handlers) GetPendingPhotos(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	user, err := h.userRepo.GetByID(userID)
	if err != nil || !isAdminUser(user) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "admin only"})
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

	photos, err := h.photoRepo.ListPendingPublicPhotos(limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch pending photos"})
	}
	if photos == nil {
		photos = []*models.Photo{}
	}

	return c.JSON(http.StatusOK, photos)
}

func (h *Handlers) ApprovePhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	user, err := h.userRepo.GetByID(userID)
	if err != nil || !isAdminUser(user) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "admin only"})
	}

	photoID, err := parseIDParam(c, "id")
	if err != nil || photoID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}

	if err := h.photoRepo.ApprovePhoto(photoID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to approve photo"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "photo approved"})
}

func (h *Handlers) RejectPhoto(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	user, err := h.userRepo.GetByID(userID)
	if err != nil || !isAdminUser(user) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "admin only"})
	}

	photoID, err := parseIDParam(c, "id")
	if err != nil || photoID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid photo id"})
	}

	var req map[string]string
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	reason := req["reason"]

	if err := h.photoRepo.RejectPhoto(photoID, reason); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to reject photo"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "photo rejected"})
}

func (h *Handlers) GetCommentReports(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	user, err := h.userRepo.GetByID(userID)
	if err != nil || !isAdminUser(user) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "admin only"})
	}

	status := c.QueryParam("status")
	if status == "" {
		status = "pending"
	}

	reports, err := h.commentRepo.GetCommentReports(status)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch reports"})
	}
	if reports == nil {
		reports = []*models.CommentReport{}
	}

	return c.JSON(http.StatusOK, reports)
}

func (h *Handlers) ResolveCommentReport(c echo.Context) error {
	userID, ok := getUserID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	user, err := h.userRepo.GetByID(userID)
	if err != nil || !isAdminUser(user) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "admin only"})
	}

	reportID, err := parseIDParam(c, "report_id")
	if err != nil || reportID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid report id"})
	}

	var req map[string]string
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	action := req["action"] // "delete" или "dismiss"
	adminNote := req["admin_note"]

	if action == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "action is required"})
	}

	if err := h.commentRepo.ResolveReport(reportID, action, adminNote); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to resolve report"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "report resolved"})
}
