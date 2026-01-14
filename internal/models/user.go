package models

type User struct {
	ID       int64  `json:"id"`
	IsAdmin  bool   `json:"is_admin"`
	Username string `json:"username"`
}
