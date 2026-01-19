package repository

import (
	"database/sql"
	"errors"
	"log"

	"github.com/freshtea599/PhotoHubServer.git/internal/models"
)

type PhotoRepository struct {
	db *sql.DB
}

func NewPhotoRepository(db *sql.DB) *PhotoRepository {
	return &PhotoRepository{db: db}
}

func (r *PhotoRepository) Create(photo *models.Photo) (*models.Photo, error) {
	isPending := photo.IsPublic

	err := r.db.QueryRow(`
		INSERT INTO photos (user_id, url, file_path, file_size, mime_type, description, is_public, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`, photo.UserID, photo.URL, photo.FilePath, photo.FileSize, photo.MimeType, photo.Description, photo.IsPublic).Scan(
		&photo.ID, &photo.CreatedAt, &photo.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if isPending {
		_, err := r.db.Exec(`
			INSERT INTO photo_statuses (photo_id, status, created_at, updated_at)
			VALUES ($1, 'pending', NOW(), NOW())
		`, photo.ID)
		if err != nil {
			log.Printf("Failed to create photo status: %v", err)
		}
		photo.IsPending = isPending
	}

	return photo, nil
}

// сохраняет вариант фото в БД
func (r *PhotoRepository) CreateVariant(variant *models.PhotoVariant) error {
	_, err := r.db.Exec(`
		INSERT INTO photo_variants (photo_id, size_name, format, file_path, width, height, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, variant.PhotoID, variant.SizeName, variant.Format, variant.FilePath, variant.Width, variant.Height)
	return err
}

func (r *PhotoRepository) ListPublic(limit, offset int) ([]*models.Photo, error) {
	rows, err := r.db.Query(`
		SELECT 
			p.id, p.user_id, p.url, p.file_path, p.file_size, 
			p.mime_type, p.description, p.is_public, p.likes_count, 
			p.created_at, p.updated_at
		FROM photos p
		LEFT JOIN photo_statuses ps ON p.id = ps.photo_id
		WHERE p.is_public = true 
		  AND (ps.status IS NULL OR ps.status = 'approved')
		ORDER BY p.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []*models.Photo
	photosMap := make(map[int64]*models.Photo)
	var photoIDs []int64

	for rows.Next() {
		var photo models.Photo
		if err := rows.Scan(
			&photo.ID, &photo.UserID, &photo.URL, &photo.FilePath, &photo.FileSize,
			&photo.MimeType, &photo.Description, &photo.IsPublic, &photo.LikesCount,
			&photo.CreatedAt, &photo.UpdatedAt,
		); err != nil {
			return nil, err
		}
		photos = append(photos, &photo)
		photosMap[photo.ID] = &photo
		photoIDs = append(photoIDs, photo.ID)
	}

	if len(photos) == 0 {
		return []*models.Photo{}, nil
	}

	err = r.enrichPhotosWithVariants(photosMap, photoIDs)
	if err != nil {
		log.Printf("Error fetching variants: %v", err)
	}

	return photos, nil
}

// Используем указатели правильно
func (r *PhotoRepository) enrichPhotosWithVariants(photosMap map[int64]*models.Photo, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	query := `
		SELECT photo_id, size_name, format, file_path, width, height
		FROM photo_variants
		WHERE photo_id = ANY($1)
		ORDER BY photo_id, size_name
	`

	rows, err := r.db.Query(query, ids)
	if err != nil {
		log.Printf("Warning: could not fetch variants: %v", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		// Создаем указатель (pointer) на структуру
		v := &models.PhotoVariant{}

		if err := rows.Scan(&v.PhotoID, &v.SizeName, &v.Format, &v.FilePath, &v.Width, &v.Height); err != nil {
			log.Printf("Error scanning variant: %v", err)
			continue
		}

		if p, ok := photosMap[v.PhotoID]; ok {
			// Теперь append работает, т.к. v это указатель (*models.PhotoVariant)
			p.Variants = append(p.Variants, v)
		}
	}

	return rows.Err()
}

func (r *PhotoRepository) GetByID(id int64) (*models.Photo, error) {
	var photo models.Photo
	err := r.db.QueryRow(`
		SELECT id, user_id, url, file_path, file_size, mime_type, description, is_public, likes_count, created_at, updated_at
		FROM photos WHERE id = $1
	`, id).Scan(
		&photo.ID, &photo.UserID, &photo.URL, &photo.FilePath, &photo.FileSize,
		&photo.MimeType, &photo.Description, &photo.IsPublic, &photo.LikesCount,
		&photo.CreatedAt, &photo.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("photo not found")
		}
		return nil, err
	}

	if photo.IsPublic {
		var status string
		err := r.db.QueryRow(`SELECT status FROM photo_statuses WHERE photo_id = $1 ORDER BY created_at DESC LIMIT 1`, id).Scan(&status)
		if err == nil && status != "approved" {
			photo.IsPending = true
		}
	}

	r.enrichPhotosWithVariants(map[int64]*models.Photo{photo.ID: &photo}, []int64{photo.ID})

	return &photo, nil
}

func (r *PhotoRepository) ListByUser(userID int64, limit, offset int) ([]*models.Photo, error) {
	rows, err := r.db.Query(`
		SELECT id, user_id, url, file_path, file_size, mime_type, description, is_public, likes_count, created_at, updated_at
		FROM photos WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []*models.Photo
	photosMap := make(map[int64]*models.Photo)
	var photoIDs []int64

	for rows.Next() {
		var p models.Photo
		rows.Scan(&p.ID, &p.UserID, &p.URL, &p.FilePath, &p.FileSize, &p.MimeType, &p.Description, &p.IsPublic, &p.LikesCount, &p.CreatedAt, &p.UpdatedAt)
		photos = append(photos, &p)
		photosMap[p.ID] = &p
		photoIDs = append(photoIDs, p.ID)
	}
	r.enrichPhotosWithVariants(photosMap, photoIDs)
	return photos, nil
}

func (r *PhotoRepository) Update(id int64, req models.UpdatePhotoRequest) (*models.Photo, error) {
	_, err := r.db.Exec(`UPDATE photos SET description = $1, is_public = $2, updated_at = NOW() WHERE id = $3`, req.Description, req.IsPublic, id)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *PhotoRepository) Delete(id int64) error {
	res, err := r.db.Exec(`DELETE FROM photos WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("photo not found")
	}
	return nil
}

func (r *PhotoRepository) LikePhoto(photoID, userID int64) error {
	_, err := r.db.Exec(`INSERT INTO photo_likes (photo_id, user_id, created_at) VALUES ($1, $2, NOW()) ON CONFLICT DO NOTHING`, photoID, userID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`UPDATE photos SET likes_count = (SELECT COUNT(*) FROM photo_likes WHERE photo_id = $1) WHERE id = $1`, photoID)
	return err
}

func (r *PhotoRepository) UnlikePhoto(photoID, userID int64) error {
	_, err := r.db.Exec(`DELETE FROM photo_likes WHERE photo_id = $1 AND user_id = $2`, photoID, userID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`UPDATE photos SET likes_count = (SELECT COUNT(*) FROM photo_likes WHERE photo_id = $1) WHERE id = $1`, photoID)
	return err
}

func (r *PhotoRepository) IsPhotoLikedByUser(photoID, userID int64) (bool, error) {
	var c int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM photo_likes WHERE photo_id = $1 AND user_id = $2`, photoID, userID).Scan(&c)
	return c > 0, err
}

func (r *PhotoRepository) ListPendingPublicPhotos(limit, offset int) ([]*models.Photo, error) {
	rows, err := r.db.Query(`
		SELECT p.id, p.user_id, p.url, p.file_path, p.file_size, p.mime_type, p.description, p.is_public, p.likes_count, p.created_at, p.updated_at
		FROM photos p JOIN photo_statuses ps ON ps.photo_id = p.id
		WHERE p.is_public = true AND ps.status = 'pending'
		ORDER BY ps.created_at ASC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var photos []*models.Photo
	for rows.Next() {
		var p models.Photo
		rows.Scan(&p.ID, &p.UserID, &p.URL, &p.FilePath, &p.FileSize, &p.MimeType, &p.Description, &p.IsPublic, &p.LikesCount, &p.CreatedAt, &p.UpdatedAt)
		p.IsPending = true
		photos = append(photos, &p)
	}
	return photos, nil
}

func (r *PhotoRepository) ApprovePhoto(photoID int64) error {
	_, err := r.db.Exec(`UPDATE photo_statuses SET status = 'approved', updated_at = NOW() WHERE photo_id = $1 AND status = 'pending'`, photoID)
	return err
}

func (r *PhotoRepository) RejectPhoto(photoID int64, reason string) error {
	_, err := r.db.Exec(`UPDATE photo_statuses SET status = 'rejected', reason = $2, updated_at = NOW() WHERE photo_id = $1 AND status = 'pending'`, photoID, reason)
	return err
}
