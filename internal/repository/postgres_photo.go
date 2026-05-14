package repository

import (
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/freshtea599/PhotoHubServer.git/internal/domain"
	"github.com/lib/pq"
)

type PostgresPhotoRepo struct {
	db *sql.DB
}

func NewPostgresPhotoRepo(db *sql.DB) *PostgresPhotoRepo {
	return &PostgresPhotoRepo{db: db}
}

// Create вставляет новое изображение
func (r *PostgresPhotoRepo) Create(photo *domain.Photo) (*domain.Photo, error) {
	err := r.db.QueryRow(`
        INSERT INTO photos (user_id, url, file_path, file_size, mime_type, description, is_public,
            blurhash, content_hash, width, height, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
        RETURNING id, created_at, updated_at
    `, photo.UserID, photo.URL, photo.FilePath, photo.FileSize, photo.MimeType,
		photo.Description, photo.IsPublic, photo.BlurHash, photo.ContentHash,
		photo.Width, photo.Height,
	).Scan(&photo.ID, &photo.CreatedAt, &photo.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return photo, nil
}

// CreateVariant сохраняет вариант
func (r *PostgresPhotoRepo) CreateVariant(variant *domain.PhotoVariant) error {
	_, err := r.db.Exec(`
        INSERT INTO photo_variants (photo_id, size_name, format, file_path, width, height, quality, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
    `, variant.PhotoID, variant.SizeName, variant.Format, variant.FilePath,
		variant.Width, variant.Height, variant.Quality)
	return err
}

// ListPublic возвращает публичные фото
func (r *PostgresPhotoRepo) ListPublic(limit, offset int) ([]*domain.Photo, error) {
	rows, err := r.db.Query(`
        SELECT id, user_id, url, file_path, file_size, mime_type, description, is_public,
            blurhash, content_hash, width, height, created_at, updated_at
        FROM photos
        WHERE is_public = true
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2
    `, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []*domain.Photo
	photosMap := make(map[int64]*domain.Photo)
	var ids []int64

	for rows.Next() {
		var p domain.Photo
		err := rows.Scan(&p.ID, &p.UserID, &p.URL, &p.FilePath, &p.FileSize,
			&p.MimeType, &p.Description, &p.IsPublic, &p.BlurHash, &p.ContentHash,
			&p.Width, &p.Height, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		photos = append(photos, &p)
		photosMap[p.ID] = &p
		ids = append(ids, p.ID)
	}
	if len(photos) > 0 {
		if err := r.enrichPhotosWithVariants(photosMap, ids); err != nil {
			log.Printf("Warning: could not load variants: %v", err)
		}
	}
	return photos, nil
}

// GetByID возвращает фото по ID
func (r *PostgresPhotoRepo) GetByID(id int64) (*domain.Photo, error) {
	var p domain.Photo
	err := r.db.QueryRow(`
        SELECT id, user_id, url, file_path, file_size, mime_type, description, is_public,
            blurhash, content_hash, width, height, created_at, updated_at
        FROM photos WHERE id = $1
    `, id).Scan(&p.ID, &p.UserID, &p.URL, &p.FilePath, &p.FileSize,
		&p.MimeType, &p.Description, &p.IsPublic, &p.BlurHash, &p.ContentHash,
		&p.Width, &p.Height, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("photo not found")
		}
		return nil, err
	}
	photosMap := map[int64]*domain.Photo{p.ID: &p}
	if err := r.enrichPhotosWithVariants(photosMap, []int64{p.ID}); err != nil {
		log.Printf("Warning: could not load variants for photo %d: %v", p.ID, err)
	}
	return &p, nil
}

// ListByUser возвращает фото пользователя
func (r *PostgresPhotoRepo) ListByUser(userID int64, limit, offset int) ([]*domain.Photo, error) {
	rows, err := r.db.Query(`
        SELECT id, user_id, url, file_path, file_size, mime_type, description, is_public,
            blurhash, content_hash, width, height, created_at, updated_at
        FROM photos
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []*domain.Photo
	photosMap := make(map[int64]*domain.Photo)
	var ids []int64

	for rows.Next() {
		var p domain.Photo
		if err := rows.Scan(&p.ID, &p.UserID, &p.URL, &p.FilePath, &p.FileSize,
			&p.MimeType, &p.Description, &p.IsPublic, &p.BlurHash, &p.ContentHash,
			&p.Width, &p.Height, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		photos = append(photos, &p)
		photosMap[p.ID] = &p
		ids = append(ids, p.ID)
	}
	if len(photos) > 0 {
		if err := r.enrichPhotosWithVariants(photosMap, ids); err != nil {
			log.Printf("Warning: could not load variants: %v", err)
		}
	}
	return photos, nil
}

// Update обновляет описание и публичность
func (r *PostgresPhotoRepo) Update(id int64, req domain.UpdatePhotoRequest) (*domain.Photo, error) {
	_, err := r.db.Exec(`
        UPDATE photos SET description = $1, is_public = $2, updated_at = NOW()
        WHERE id = $3
    `, req.Description, req.IsPublic, id)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

// Delete удаляет фото
func (r *PostgresPhotoRepo) Delete(id int64) error {
	res, err := r.db.Exec(`DELETE FROM photos WHERE id = $1`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("photo not found")
	}
	return nil
}

// enrichPhotosWithVariants подгружает варианты
func (r *PostgresPhotoRepo) enrichPhotosWithVariants(photosMap map[int64]*domain.Photo, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := r.db.Query(`
        SELECT id, photo_id, size_name, format, file_path, width, height, quality, created_at
        FROM photo_variants
        WHERE photo_id = ANY($1)
        ORDER BY photo_id, size_name
    `, pq.Array(ids))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &domain.PhotoVariant{}
		if err := rows.Scan(&v.ID, &v.PhotoID, &v.SizeName, &v.Format, &v.FilePath,
			&v.Width, &v.Height, &v.Quality, &v.CreatedAt); err != nil {
			log.Printf("Error scanning variant: %v", err)
			continue
		}
		if p, ok := photosMap[v.PhotoID]; ok {
			p.Variants = append(p.Variants, v)
		}
	}
	return rows.Err()
}

// ===================== НОВЫЕ МЕТОДЫ =====================

// LikePhoto добавляет лайк пользователя к фото
func (r *PostgresPhotoRepo) LikePhoto(photoID, userID int64) error {
	_, err := r.db.Exec("INSERT INTO photo_likes (photo_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", photoID, userID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec("UPDATE photos SET likes_count = COALESCE(likes_count,0)+1 WHERE id = $1", photoID)
	return err
}

// UnlikePhoto удаляет лайк пользователя
func (r *PostgresPhotoRepo) UnlikePhoto(photoID, userID int64) error {
	_, err := r.db.Exec("DELETE FROM photo_likes WHERE photo_id = $1 AND user_id = $2", photoID, userID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec("UPDATE photos SET likes_count = GREATEST(COALESCE(likes_count,0)-1, 0) WHERE id = $1", photoID)
	return err
}

// IsPhotoLiked проверяет, лайкнул ли пользователь фото
func (r *PostgresPhotoRepo) IsPhotoLiked(photoID, userID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow("SELECT EXISTS(SELECT 1 FROM photo_likes WHERE photo_id=$1 AND user_id=$2)", photoID, userID).Scan(&exists)
	return exists, err
}

// Comment структура для комментария
type Comment struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	PhotoID   int64     `json:"photo_id"`
	Text      string    `json:"text"`
	Likes     int       `json:"likes_count"`
	CreatedAt time.Time `json:"created_at"`
	Username  string    `json:"username"`
}

// CreateComment создаёт новый комментарий
func (r *PostgresPhotoRepo) CreateComment(photoID, userID int64, text string) (int64, error) {
	var id int64
	err := r.db.QueryRow(`
        INSERT INTO comments (photo_id, user_id, text, created_at, updated_at)
        VALUES ($1, $2, $3, NOW(), NOW())
        RETURNING id
    `, photoID, userID, text).Scan(&id)
	return id, err
}

// GetComments возвращает комментарии для фото
func (r *PostgresPhotoRepo) GetComments(photoID int64) ([]Comment, error) {
	rows, err := r.db.Query(`
        SELECT c.id, c.user_id, c.text, c.likes_count, c.created_at, u.username
        FROM comments c
        JOIN users u ON c.user_id = u.id
        WHERE c.photo_id = $1
        ORDER BY c.created_at DESC
    `, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.UserID, &c.Text, &c.Likes, &c.CreatedAt, &c.Username); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}

// LikeComment добавляет лайк комментарию
func (r *PostgresPhotoRepo) LikeComment(commentID, userID int64) error {
	_, err := r.db.Exec("INSERT INTO comment_likes (comment_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", commentID, userID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec("UPDATE comments SET likes_count = COALESCE(likes_count,0)+1 WHERE id = $1", commentID)
	return err
}

// UnlikeComment удаляет лайк комментария
func (r *PostgresPhotoRepo) UnlikeComment(commentID, userID int64) error {
	_, err := r.db.Exec("DELETE FROM comment_likes WHERE comment_id = $1 AND user_id = $2", commentID, userID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec("UPDATE comments SET likes_count = GREATEST(COALESCE(likes_count,0)-1, 0) WHERE id = $1", commentID)
	return err
}

// ReportComment создаёт жалобу на комментарий
func (r *PostgresPhotoRepo) ReportComment(commentID, userID int64, reason string) error {
	_, err := r.db.Exec(`
        INSERT INTO comment_reports (comment_id, reported_by, reason, status, created_at, updated_at)
        VALUES ($1, $2, $3, 'pending', NOW(), NOW())
    `, commentID, userID, reason)
	return err
}

// PendingPhoto для админки
type PendingPhoto struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	URL         string    `json:"url"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	Username    string    `json:"username"`
}

// GetPendingPhotos возвращает непроверенные публичные фото (is_public = false)
func (r *PostgresPhotoRepo) GetPendingPhotos() ([]PendingPhoto, error) {
	rows, err := r.db.Query(`
        SELECT p.id, p.user_id, p.url, p.description, p.created_at, u.username
        FROM photos p
        JOIN users u ON p.user_id = u.id
        WHERE p.is_public = false
        ORDER BY p.created_at ASC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var photos []PendingPhoto
	for rows.Next() {
		var ph PendingPhoto
		if err := rows.Scan(&ph.ID, &ph.UserID, &ph.URL, &ph.Description, &ph.CreatedAt, &ph.Username); err != nil {
			return nil, err
		}
		photos = append(photos, ph)
	}
	return photos, nil
}

// ApprovePhoto делает фото публичным
func (r *PostgresPhotoRepo) ApprovePhoto(photoID int64) error {
	_, err := r.db.Exec("UPDATE photos SET is_public = true, updated_at = NOW() WHERE id = $1", photoID)
	return err
}

// RejectPhoto удаляет фото (отклоняет)
func (r *PostgresPhotoRepo) RejectPhoto(photoID int64) error {
	_, err := r.db.Exec("DELETE FROM photos WHERE id = $1", photoID)
	return err
}
