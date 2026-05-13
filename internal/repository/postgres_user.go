// backend/internal/repository/postgres_user.go
package repository

import (
	"database/sql"
	"errors"

	"github.com/freshtea599/PhotoHubServer.git/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

// PostgresUserRepo работает с таблицей users.
type PostgresUserRepo struct {
	db *sql.DB
}

// NewPostgresUserRepo создаёт новый экземпляр репозитория.
func NewPostgresUserRepo(db *sql.DB) *PostgresUserRepo {
	return &PostgresUserRepo{db: db}
}

// Create регистрирует нового пользователя и возвращает его.
func (r *PostgresUserRepo) Create(email, password, username string) (*domain.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var user domain.User
	err = r.db.QueryRow(`
		INSERT INTO users (email, password_hash, username, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, email, username, created_at, updated_at
	`, email, string(hashedPassword), username).Scan(
		&user.ID, &user.Email, &user.Username, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByEmail возвращает пользователя вместе с хешем пароля.
func (r *PostgresUserRepo) GetByEmail(email string) (*domain.User, string, error) {
	var user domain.User
	var passwordHash string

	err := r.db.QueryRow(`
		SELECT id, email, username, is_admin, password_hash, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(&user.ID, &user.Email, &user.Username, &user.IsAdmin,
		&passwordHash, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", errors.New("user not found")
		}
		return nil, "", err
	}
	return &user, passwordHash, nil
}

// GetByID возвращает пользователя по ID.
func (r *PostgresUserRepo) GetByID(id int64) (*domain.User, error) {
	var user domain.User
	err := r.db.QueryRow(`
		SELECT id, email, username, is_admin, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&user.ID, &user.Email, &user.Username, &user.IsAdmin,
		&user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// CheckPassword проверяет соответствие пароля для указанного email.
func (r *PostgresUserRepo) CheckPassword(email, password string) (*domain.User, error) {
	user, hash, err := r.GetByEmail(email)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, errors.New("invalid password")
	}
	return user, nil
}
