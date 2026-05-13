// backend/internal/domain/user.go
package domain

import "time"

// User представляет пользователя системы.
type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RegisterRequest – данные для регистрации.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Username string `json:"username"`
}

// LoginRequest – данные для входа.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse – ответ с токеном и данными пользователя.
type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}
