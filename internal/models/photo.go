package models

import (
	"database/sql"
	"time"
)

type Photo struct {
	ID          int64         `json:"id"`
	UserID      int64         `json:"user_id"`
	URL         string        `json:"url"`
	FilePath    string        `json:"file_path"`
	FileSize    sql.NullInt64 `json:"file_size"`
	MimeType    string        `json:"mime_type"`
	Description string        `json:"description"`
	IsPublic    bool          `json:"is_public"`
	IsPending   bool          `json:"is_pending"` // новое поле: ждёт проверки админа
	LikesCount  int64         `json:"likes_count"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type UpdatePhotoRequest struct {
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

type Comment struct {
	ID         int64     `json:"id"`
	PhotoID    int64     `json:"photo_id"`
	UserID     int64     `json:"user_id"`
	Username   string    `json:"username"`
	Text       string    `json:"text"`
	LikesCount int64     `json:"likes_count"`
	UserLiked  bool      `json:"user_liked"` // лайкнул ли текущий юзер
	CreatedAt  time.Time `json:"created_at"`
}

type CreateCommentRequest struct {
	Text string `json:"text" binding:"required"`
}

type CommentReport struct {
	ID         int64     `json:"id"`
	CommentID  int64     `json:"comment_id"`
	ReportedBy int64     `json:"reported_by"`
	Reason     string    `json:"reason"`
	Status     string    `json:"status"`
	AdminNote  string    `json:"admin_note"`
	Comment    Comment   `json:"comment"`
	CreatedAt  time.Time `json:"created_at"`
}

type PhotoStatus struct {
	ID      int64  `json:"id"`
	PhotoID int64  `json:"photo_id"`
	Status  string `json:"status"` // pending, approved, rejected
	Reason  string `json:"reason"`
}

type ReportCommentRequest struct {
	Reason string `json:"reason"`
}
type PhotoVariant struct {
	ID        int64     `json:"id"`
	PhotoID   int64     `json:"photo_id"`
	SizeName  string    `json:"size_name"` // thumb, small, medium, large, fullscreen
	Format    string    `json:"format"`    // webp, jpeg
	FilePath  string    `json:"file_path"`
	FileSize  int64     `json:"file_size"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Quality   int       `json:"quality"`
	CreatedAt time.Time `json:"created_at"`
}

// PhotoWithVariants - Photo с массивом вариантов
type PhotoWithVariants struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	LikesCount  int64     `json:"likes_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Новые поля
	ImageID      string `json:"image_id"`
	OriginalSize int64  `json:"original_size"`
	MimeType     string `json:"mime_type"`

	// Варианты
	Variants []*PhotoVariant `json:"variants"`
}

// ImagePipelineConfig - конфигурация pipeline'а
type ImagePipelineConfig struct {
	Enabled         bool
	WebPEnabled     bool
	Quality         int      // 75–90
	GenerateSizes   []string // ["thumb", "small", "medium", "large", "fullscreen"]
	AsyncProcessing bool
	ThumbSize       int // 300
	SmallSize       int // 480
	MediumSize      int // 768
	LargeSize       int // 1200
}
