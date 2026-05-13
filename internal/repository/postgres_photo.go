// backend/internal/repository/postgres_photo.go
package repository

import (
	"database/sql"
	"errors"
	"log"

	"github.com/freshtea599/PhotoHubServer.git/internal/domain"
)

// PostgresPhotoRepo работает с таблицей photos и photo_variants.
type PostgresPhotoRepo struct {
	db *sql.DB
}

// NewPostgresPhotoRepo создаёт новый экземпляр репозитория.
func NewPostgresPhotoRepo(db *sql.DB) *PostgresPhotoRepo {
	return &PostgresPhotoRepo{db: db}
}

// Create вставляет новое изображение, возвращает заполненный объект с ID.
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

// CreateVariant сохраняет запись о сгенерированном варианте.
func (r *PostgresPhotoRepo) CreateVariant(variant *domain.PhotoVariant) error {
	_, err := r.db.Exec(`
		INSERT INTO photo_variants (photo_id, size_name, format, file_path, width, height, quality, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`, variant.PhotoID, variant.SizeName, variant.Format, variant.FilePath,
		variant.Width, variant.Height, variant.Quality)
	return err
}

// ListPublic возвращает все публичные изображения с пагинацией.
// Для простоты прототипа не используем модерацию, просто is_public = true.
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

// GetByID возвращает фото по ID с вариантами.
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

	// подгружаем варианты
	photosMap := map[int64]*domain.Photo{p.ID: &p}
	if err := r.enrichPhotosWithVariants(photosMap, []int64{p.ID}); err != nil {
		log.Printf("Warning: could not load variants for photo %d: %v", p.ID, err)
	}
	return &p, nil
}

// ListByUser возвращает все фото конкретного пользователя.
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

// Update обновляет описание и/или флаг публичности.
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

// Delete удаляет фото. (Внешние ключи CASCADE позаботятся об удалении вариантов.)
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

// enrichPhotosWithVariants подгружает все варианты для указанных photoID и добавляет их в объекты Photo.
func (r *PostgresPhotoRepo) enrichPhotosWithVariants(photosMap map[int64]*domain.Photo, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := r.db.Query(`
		SELECT id, photo_id, size_name, format, file_path, width, height, quality, created_at
		FROM photo_variants
		WHERE photo_id = ANY($1)
		ORDER BY photo_id, size_name
	`, ids)
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
