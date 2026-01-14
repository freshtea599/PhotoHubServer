package models

type Photo struct {
	ID          int64  `json:"id"`
	URL         string `json:"url"`
	UserID      int64  `json:"user_id"`
	Description string `json:"description"`
	LikesCount  int64  `json:"likes_count"`
	IsPublic    bool   `json:"is_public"`
}
