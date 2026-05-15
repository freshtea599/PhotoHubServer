package domain

import (
	"database/sql"
	"time"
)

type Photo struct {
	ID            int64         `json:"id"`
	UserID        int64         `json:"user_id,omitempty"`
	URL           string        `json:"url"`
	FilePath      string        `json:"file_path"`
	FileSize      sql.NullInt64 `json:"file_size"`
	MimeType      string        `json:"mime_type"`
	Description   string        `json:"description"`
	IsPublic      bool          `json:"is_public"`
	LikesCount    int           `json:"likes_count"`
	CommentsCount int           `json:"comments_count"`
	// Новые поля для методики
	BlurHash    string `json:"blurhash"`
	ContentHash string `json:"content_hash"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	// Остальные поля (убраны likes_count, comments_count и пр.)
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Variants  []*PhotoVariant `json:"variants,omitempty"`
}

type PhotoVariant struct {
	ID        int64     `json:"id"`
	PhotoID   int64     `json:"photo_id"`
	SizeName  string    `json:"size_name"`
	Format    string    `json:"format"`
	FilePath  string    `json:"file_path"`
	FileSize  int64     `json:"file_size"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Quality   int       `json:"quality"`
	CreatedAt time.Time `json:"created_at"`
}

type UpdatePhotoRequest struct {
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}
