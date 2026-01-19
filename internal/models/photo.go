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
	IsPending   bool          `json:"is_pending"`
	LikesCount  int64         `json:"likes_count"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`

	Variants []*PhotoVariant `json:"variants,omitempty"`
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
	UserLiked  bool      `json:"user_liked"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}

type ReportCommentRequest struct {
	Reason string `json:"reason"`
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

type ImagePipelineConfig struct {
	Enabled         bool
	WebPEnabled     bool
	Quality         int
	GenerateSizes   []string
	AsyncProcessing bool
	ThumbSize       int
	SmallSize       int
	MediumSize      int
	LargeSize       int
}
